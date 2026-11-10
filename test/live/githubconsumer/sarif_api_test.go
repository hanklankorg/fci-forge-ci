// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

//go:build live

package githubconsumer_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"codefloe.com/itiquette/forge-lab-contract/githubclient"
	"codefloe.com/itiquette/forge-lab-contract/githubcontext"
	githubadapter "github.com/diggsweden/forge-ci/internal/adapters/github"
	appsecurity "github.com/diggsweden/forge-ci/internal/app/security"
	"github.com/diggsweden/forge-ci/internal/domain/errs"
	"github.com/diggsweden/forge-ci/internal/domain/output"
	"github.com/diggsweden/forge-ci/internal/retry"
	"github.com/diggsweden/forge-ci/test/live/internal/livetest"
	sdk "github.com/google/go-github/v91/github"
)

// The one finding the fixture report carries, and where Code Scanning must
// file it.
const (
	sarifFixtureTool     = "forge-ci-live-sarif-fixture"
	sarifFixtureRule     = "FCI-LIVE-0001"
	sarifFixtureFile     = "sarif-fixture.txt"
	sarifAcceptedSubject = "forge-ci-live-accepted"
	sarifDeniedSubject   = "forge-ci-live-denied"
)

const sarifFixtureReport = `{
  "$schema": "https://json.schemastore.org/sarif-2.1.0.json",
  "version": "2.1.0",
  "runs": [
    {
      "tool": {
        "driver": {
          "name": "` + sarifFixtureTool + `",
          "rules": [
            {"id": "` + sarifFixtureRule + `", "shortDescription": {"text": "Live SARIF fixture finding"}}
          ]
        }
      },
      "results": [
        {
          "ruleId": "` + sarifFixtureRule + `",
          "level": "warning",
          "message": {"text": "Live SARIF fixture finding"},
          "locations": [
            {"physicalLocation": {"artifactLocation": {"uri": "` + sarifFixtureFile + `"}, "region": {"startLine": 1}}}
          ]
        }
      ]
    }
  ]
}
`

// sarifWant is the analysis Code Scanning must hold after the accepted upload.
type sarifWant struct {
	Commit, Ref, Category, Tool string
	Results                     int
}

// sarifAnalysisGap judges the analyses Code Scanning lists for the ref: it
// must hold exactly the accepted upload, processed without error, for the
// exact commit, ref, category and tool, and nothing else. An analysis in any
// other category, the denied upload's included, fails.
func sarifAnalysisGap(analyses []*sdk.ScanningAnalysis, want sarifWant) error {
	if len(analyses) != 1 {
		categories := make([]string, 0, len(analyses))
		for _, analysis := range analyses {
			categories = append(categories, analysis.GetCategory())
		}

		return fmt.Errorf("code scanning lists %d analyses %v, want exactly the %s upload", len(analyses), categories, want.Category)
	}

	got := analyses[0]

	switch {
	case got.GetError() != "":
		return fmt.Errorf("analysis %d was processed with an error: %s", got.GetID(), got.GetError())
	case got.GetCommitSHA() != want.Commit:
		return fmt.Errorf("analysis %d is for commit %s, want %s", got.GetID(), got.GetCommitSHA(), want.Commit)
	case got.GetRef() != want.Ref:
		return fmt.Errorf("analysis %d is for ref %s, want %s", got.GetID(), got.GetRef(), want.Ref)
	case got.GetCategory() != want.Category:
		return fmt.Errorf("analysis %d has category %q, want %q", got.GetID(), got.GetCategory(), want.Category)
	case got.GetTool().GetName() != want.Tool:
		return fmt.Errorf("analysis %d is by tool %q, want %q", got.GetID(), got.GetTool().GetName(), want.Tool)
	case got.GetResultsCount() != want.Results:
		return fmt.Errorf("analysis %d holds %d results, want %d", got.GetID(), got.GetResultsCount(), want.Results)
	}

	return nil
}

// sarifAlertGap judges the open alerts of the fixture tool on the ref: exactly
// the fixture's one finding, by its rule, in its file, at the commit.
func sarifAlertGap(alerts []*sdk.Alert, commit string) error {
	if len(alerts) != 1 {
		return fmt.Errorf("code scanning lists %d open alerts of %s, want 1", len(alerts), sarifFixtureTool)
	}

	alert, instance := alerts[0], alerts[0].GetMostRecentInstance()

	switch {
	case alert.GetRule().GetID() != sarifFixtureRule:
		return fmt.Errorf("alert %d is for rule %q, want %q", alert.GetNumber(), alert.GetRule().GetID(), sarifFixtureRule)
	case instance.GetLocation().GetPath() != sarifFixtureFile:
		return fmt.Errorf("alert %d is in %q, want %q", alert.GetNumber(), instance.GetLocation().GetPath(), sarifFixtureFile)
	case instance.GetCommitSHA() != commit:
		return fmt.Errorf("alert %d is at commit %s, want %s", alert.GetNumber(), instance.GetCommitSHA(), commit)
	}

	return nil
}

// codeScanningUnavailable names why GitHub refuses Code Scanning to this
// repository or token as such: the plan or the repository's settings do not
// offer it, or the session credential lacks the permission. Any other
// refusal, a rejected credential included, is not one of these and fails the
// scenario. The phrases are GitHub's own, read from the product's error.
func codeScanningUnavailable(err error) (string, bool) {
	if !errors.Is(err, errs.ErrPermissionDenied) {
		return "", false
	}

	text := strings.ToLower(err.Error())

	for _, refusal := range [][2]string{
		{"advanced security must be enabled", "the organization's plan offers no Code Scanning for this repository (GitHub Advanced Security is not enabled)"},
		{"code security must be enabled", "the organization's plan offers no Code Scanning for this repository (GitHub Code Security is not enabled)"},
		{"code scanning is not enabled", "Code Scanning is not enabled for this repository"},
		{"resource not accessible by", "the session credential lacks the Code scanning alerts permission"},
	} {
		if strings.Contains(text, refusal[0]) {
			return refusal[1], true
		}
	}

	return "", false
}

// Positive SARIF ingestion on hosted GitHub, which no lab forge offers: the
// product's own upload (`security report upload-sarif`'s use case over the
// GitHub adapter, not an upload action) submits one tiny report for an exact
// repository, commit, ref and category; Code Scanning must then list exactly
// that analysis and its one alert. A second upload with a credential GitHub
// rejects must be denied, leak nothing and leave no analysis. No runner,
// engine push or scanner is involved.
//
// Code Scanning is free on a public repository and needs GitHub Advanced
// Security on a private one. Where the plan, the repository or the session
// credential's permissions prohibit it, the case is reported not covered with
// that reason, never passed.
func TestGitHubSecurity_SARIFUploadAPI(t *testing.T) {
	livetest.RegisterLiveScenario(t)

	path := os.Getenv(livetest.GitHubContextEnv)
	if path == "" {
		t.Skip("run inside Forge Lab github run --fixtures none")
	}

	id, err := strconv.ParseInt(os.Getenv(organizationEnv), 10, 64)
	if err != nil || id <= 0 {
		t.Fatal("explicit GitHub organization ID is required")
	}

	document, err := githubcontext.Load(path)
	if err != nil {
		t.Fatal(err)
	}

	client, err := githubclient.New(livetest.Context(t), document, id)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(client.Close)
	api, owner := client.API(), document.Organization.Login

	for _, visibility := range livetest.ConsumerVisibilities(t) {
		t.Run(visibility.Name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(livetest.Context(t), 5*time.Minute)
			defer cancel()

			name := document.Prefix + "sarif-api-" + visibility.Name + "-" + document.Session[:8]

			repo, err := client.CreateRepository(ctx, name, visibility.Private)
			if repo != nil && repo.GetID() > 0 {
				repoID := repo.GetID()
				t.Logf("disposable repository: %s/%s id=%d", owner, name, repoID)
				t.Cleanup(func() {
					cleanup, stop := context.WithTimeout(context.WithoutCancel(t.Context()), time.Minute)
					defer stop()

					if err := client.DeleteRepository(cleanup, name, repoID); err != nil {
						t.Errorf("repository cleanup: %v", err)

						return
					}

					if _, response, err := api.Repositories.GetByID(cleanup, repoID); err == nil || response == nil || response.StatusCode != http.StatusNotFound {
						t.Error("repository deletion not confirmed")
					}
				})
			}

			if err != nil {
				t.Fatal(err)
			}

			seed, _, err := api.Repositories.CreateFile(ctx, owner, name, sarifFixtureFile, &sdk.RepositoryContentFileOptions{Message: new("test: seed disposable SARIF fixture"), Content: []byte("fixture\n")})
			if err != nil {
				t.Fatal(err)
			}

			current, _, err := api.Repositories.GetByID(ctx, repo.GetID())
			if err != nil {
				t.Fatalf("read created repository identity: %v", err)
			}

			if current.GetName() != name || current.GetOwner().GetID() != id || current.Private == nil || current.GetPrivate() != visibility.Private || current.GetDefaultBranch() == "" {
				t.Fatal("scratch repository identity changed")
			}

			report := filepath.Join(t.TempDir(), "fixture.sarif")
			if err := os.WriteFile(report, []byte(sarifFixtureReport), 0o600); err != nil {
				t.Fatal(err)
			}

			want := sarifWant{Commit: seed.GetSHA(), Ref: "refs/heads/" + current.GetDefaultBranch(), Category: sarifAcceptedSubject, Tool: sarifFixtureTool, Results: 1}

			// The product's upload: the use case and adapter the CLI command runs.
			upload := func(token, category string) (string, error) {
				var notes bytes.Buffer

				err := appsecurity.UploadSARIF(ctx, &githubadapter.Provider{Env: func(string) string { return "" }}, io.Discard,
					output.NewAnnotator(&notes, output.FormatText), appsecurity.UploadSARIFInput{
						SARIFFile: report, Token: token, Repository: owner + "/" + name, SHA: want.Commit, Ref: want.Ref, Category: category,
					})

				text := notes.String()
				if err != nil {
					text += err.Error()
				}

				return text, err
			}

			if said, err := upload(document.Token, want.Category); err != nil {
				if strings.Contains(said, document.Token) {
					t.Fatal("the refused upload's diagnostics carry the session credential")
				}

				if reason, unavailable := codeScanningUnavailable(err); unavailable {
					livetest.NotCoveredf(t, "SARIF ingestion on a %s repository of %s: %s", visibility.Name, owner, reason)
				}

				t.Fatalf("the product's SARIF upload was refused: %v", err)
			}

			// The controlled denial: a credential GitHub does not know. It is
			// random, so nothing it could leak is a secret.
			random := make([]byte, 20)
			if _, err := rand.Read(random); err != nil {
				t.Fatal(err)
			}

			rejected := "forge-ci-live-rejected-" + hex.EncodeToString(random)

			said, err := upload(rejected, sarifDeniedSubject)
			if !errors.Is(err, errs.ErrPermissionDenied) {
				t.Fatalf("an upload with a rejected credential was not denied: %v", err)
			}

			if strings.Contains(said, rejected) {
				t.Fatal("the denied upload's diagnostics carry its credential")
			}

			// Processing is asynchronous: GitHub answers the upload before the
			// analysis is listed, and lists none (404) until it is.
			if err := retry.Run(ctx, nil, 36, 5*time.Second, func() error {
				analyses, response, err := api.CodeScanning.ListAnalysesForRepo(ctx, owner, name, &sdk.AnalysesListOptions{Ref: &want.Ref})
				if err != nil && (response == nil || response.StatusCode != http.StatusNotFound) {
					return retry.Permanent(fmt.Errorf("read the analyses back: %w", err))
				}

				if gap := sarifAnalysisGap(analyses, want); gap != nil {
					return gap
				}

				alerts, _, err := api.CodeScanning.ListAlertsForRepo(ctx, owner, name, &sdk.AlertListOptions{State: "open", Ref: want.Ref, ToolName: sarifFixtureTool})
				if err != nil {
					return retry.Permanent(fmt.Errorf("read the alerts back: %w", err))
				}

				return sarifAlertGap(alerts, want.Commit)
			}, retry.WithBackoff(retry.Constant)); err != nil {
				t.Fatalf("Code Scanning readback: %v", err)
			}

			t.Logf("Code Scanning holds the %s analysis of %s@%s on %s with its alert; the rejected upload left none", want.Category, name, want.Commit, want.Ref)
		})
	}
}

func TestSARIFAnalysisGap(t *testing.T) {
	want := sarifWant{Commit: "c0ffee", Ref: "refs/heads/main", Category: sarifAcceptedSubject, Tool: sarifFixtureTool, Results: 1}
	accepted := func() *sdk.ScanningAnalysis {
		return &sdk.ScanningAnalysis{
			ID: new(int64(7)), CommitSHA: new("c0ffee"), Ref: new("refs/heads/main"), Category: new(sarifAcceptedSubject),
			Tool: &sdk.Tool{Name: new(sarifFixtureTool)}, ResultsCount: new(1), Error: new(""),
		}
	}
	altered := func(alter func(*sdk.ScanningAnalysis)) *sdk.ScanningAnalysis {
		analysis := accepted()
		alter(analysis)

		return analysis
	}

	if err := sarifAnalysisGap([]*sdk.ScanningAnalysis{accepted()}, want); err != nil {
		t.Fatalf("the accepted analysis is refused: %v", err)
	}

	for name, analyses := range map[string][]*sdk.ScanningAnalysis{
		"none listed":      nil,
		"denied also held": {accepted(), altered(func(a *sdk.ScanningAnalysis) { a.Category = new(sarifDeniedSubject) })},
		"other commit":     {altered(func(a *sdk.ScanningAnalysis) { a.CommitSHA = new("decaf") })},
		"other ref":        {altered(func(a *sdk.ScanningAnalysis) { a.Ref = new("refs/heads/other") })},
		"other category":   {altered(func(a *sdk.ScanningAnalysis) { a.Category = new(sarifDeniedSubject) })},
		"other tool":       {altered(func(a *sdk.ScanningAnalysis) { a.Tool = &sdk.Tool{Name: new("other")} })},
		"no results":       {altered(func(a *sdk.ScanningAnalysis) { a.ResultsCount = new(0) })},
		"processing error": {altered(func(a *sdk.ScanningAnalysis) { a.Error = new("invalid SARIF") })},
	} {
		t.Run(name, func(t *testing.T) {
			if err := sarifAnalysisGap(analyses, want); err == nil {
				t.Fatal("a wrong readback is accepted")
			}
		})
	}
}

func TestSARIFAlertGap(t *testing.T) {
	found := func() *sdk.Alert {
		return &sdk.Alert{Number: new(1), Rule: &sdk.Rule{ID: new(sarifFixtureRule)}, MostRecentInstance: &sdk.MostRecentInstance{
			CommitSHA: new("c0ffee"), Location: &sdk.Location{Path: new(sarifFixtureFile)},
		}}
	}
	altered := func(alter func(*sdk.Alert)) *sdk.Alert {
		alert := found()
		alter(alert)

		return alert
	}

	if err := sarifAlertGap([]*sdk.Alert{found()}, "c0ffee"); err != nil {
		t.Fatalf("the fixture's alert is refused: %v", err)
	}

	for name, alerts := range map[string][]*sdk.Alert{
		"none listed":  nil,
		"two listed":   {found(), found()},
		"other rule":   {altered(func(a *sdk.Alert) { a.Rule = &sdk.Rule{ID: new("OTHER")} })},
		"other file":   {altered(func(a *sdk.Alert) { a.MostRecentInstance.Location.Path = new("other.txt") })},
		"other commit": {altered(func(a *sdk.Alert) { a.MostRecentInstance.CommitSHA = new("decaf") })},
	} {
		t.Run(name, func(t *testing.T) {
			if err := sarifAlertGap(alerts, "c0ffee"); err == nil {
				t.Fatal("a wrong alert readback is accepted")
			}
		})
	}
}

// The fixture report is what the product uploads, so it must be the SARIF the
// oracle's constants describe.
func TestSARIFFixtureReportNamesItsOracle(t *testing.T) {
	for _, needle := range []string{`"version": "2.1.0"`, `"name": "` + sarifFixtureTool + `"`, `"ruleId": "` + sarifFixtureRule + `"`, `"uri": "` + sarifFixtureFile + `"`} {
		if !strings.Contains(sarifFixtureReport, needle) {
			t.Errorf("the fixture report lacks %s", needle)
		}
	}
}

func TestCodeScanningUnavailableIsNarrow(t *testing.T) {
	denied := func(body string) error {
		return fmt.Errorf("upload sarif: HTTP 403: %s: %w", body, errs.ErrPermissionDenied)
	}

	for name, err := range map[string]error{
		"advanced security": denied(`{"message":"Advanced Security must be enabled for this repository to use code scanning."}`),
		"code security":     denied(`{"message":"Code Security must be enabled for this repository to use code scanning."}`),
		"not enabled":       denied(`{"message":"Code scanning is not enabled for this repository."}`),
		"token permission":  denied(`{"message":"Resource not accessible by personal access token"}`),
	} {
		t.Run(name, func(t *testing.T) {
			if reason, unavailable := codeScanningUnavailable(err); !unavailable || reason == "" {
				t.Fatal("a plan or permission refusal is not named as unavailable")
			}
		})
	}

	for name, err := range map[string]error{
		"accepted":            nil,
		"rejected credential": fmt.Errorf("upload sarif: HTTP 401: %s: %w", `{"message":"Bad credentials"}`, errs.ErrPermissionDenied),
		"other forbidden":     denied(`{"message":"Forbidden"}`),
		"invalid request":     fmt.Errorf("upload sarif: HTTP 422: Advanced Security must be enabled: %w", errs.ErrValidation),
		"service unavailable": fmt.Errorf("upload sarif: HTTP 502: %w", errs.ErrDependencyUnavailable),
	} {
		t.Run(name, func(t *testing.T) {
			if reason, unavailable := codeScanningUnavailable(err); unavailable {
				t.Fatalf("a product or request failure would be reported not covered: %s", reason)
			}
		})
	}
}
