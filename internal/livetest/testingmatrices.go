// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package livetest

import (
	"cmp"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/diggsweden/forge-ci/internal/domain/config"
	"github.com/diggsweden/forge-ci/internal/domain/enforcement"
	"github.com/diggsweden/forge-ci/internal/domain/errs"
	"github.com/diggsweden/forge-ci/internal/domain/pipeline"
	"github.com/diggsweden/forge-ci/internal/domain/snapshotimage"
)

// docs/testing-matrices.md keeps the testing matrices. Matrix 1, project
// kind × flow, and matrix 2, the lifecycle's variant axes, are generated here
// between the page's markers from one source, what the tree says: the shipped
// demos' declarations as the planner and the campaign admission read them,
// the cases files' lines and settings, and the live marks of
// TestingMatricesEvidence, a hand-kept overlay keyed by provider, demo and
// flow, since whether a run proved something is a judgement; SeedTestingEvidence
// drafts it from recorded verdicts. Matrix 1b, the platform capabilities, is
// hand-written. test/syncguard compares the committed page with a fresh
// rendering; `just gen-testing-matrices` refreshes it.

// TestingMatricesPage and TestingMatricesEvidence are the page and the
// overlay, relative to the repository root.
const (
	TestingMatricesPage     = "docs/testing-matrices.md"
	TestingMatricesEvidence = "test/live/campaigns/evidence.txt"
)

const (
	matrixCampaignsDir  = "test/live/campaigns"
	matrixGitHub        = "github"
	matrixDevelopment   = "development"
	matrixScenario      = "scenario"
	matrixGPGConfig     = ".forge-ci/artifacts-gpg.yml"
	matrixEvidenceCols  = 4
	routeNoContainer    = "no container"
	routeExtraction     = "extraction image"
	routeRefused        = "refused snapshot"
	routeArtifactFirst  = "artifact-first"
	routeContainerFirst = "container-first"
)

// The marks the generator writes; the overlay's marks are the legend's.
const (
	markRefused   = "⛔"
	markLine      = "📝"
	markNoLine    = "⬚"
	markNotApply  = "—"
	matrixArchRow = "amd64"
)

var errTestingMatrices = errors.New("testing matrices")

// matrixProviders are the columns. GitHub has no campaign lane: its line for
// an event is the demo's fixture workflow the hosted harness runs for it.
var matrixProviders = []string{campaignForgejo, campaignGitLab, matrixGitHub}

// githubDemoWorkflows is where a demo keeps its GitHub workflows, which a
// GitHub trial renders into .github/workflows.
const githubDemoWorkflows = "test/live/github/workflows"

// githubEvents are the hosted harness's events with a fixture workflow each;
// its lifecycle runs all three.
var githubEvents = []string{TrialPullRequest, TrialDevRelease, TrialRelease}

// matrixDemo is a shipped demo as the matrices read it.
type matrixDemo struct {
	name, source, ecosystem, route, images, signing string
	hosted                                          map[string]bool
	release                                         map[string]bool
	admission                                       map[string]error
	// snapshotRefusal is the planner's refusal of the declaration's snapshot.
	snapshotRefusal error
}

// matrixLane is one cases file.
type matrixLane struct {
	file, provider string
	settings       map[string]string
	consumers      map[string][]string
	scenarios      []string
}

// matrixMark is one overlay line's mark and note.
type matrixMark struct{ mark, note string }

// matrixEvidence maps provider/demo/event to the overlay's mark.
type matrixEvidence map[string]matrixMark

type matrices struct {
	demos    []matrixDemo
	lanes    []matrixLane
	evidence matrixEvidence
}

// RenderTestingMatrices returns page with its generated blocks regenerated
// from the tree under root.
func RenderTestingMatrices(root, page string) (string, error) {
	data, err := readMatrices(root)
	if err != nil {
		return "", err
	}

	page, err = replaceGeneratedBlock(page, "project-kinds", data.projectKindsTable())
	if err != nil {
		return "", err
	}

	return replaceGeneratedBlock(page, "e2e-variants", data.variantsSection())
}

func replaceGeneratedBlock(page, name, body string) (string, error) {
	start, end := "<!-- generated:"+name+" -->", "<!-- /generated:"+name+" -->"

	head, rest, found := strings.Cut(page, start)
	if !found {
		return "", fmt.Errorf("%w: %s has no %s", errTestingMatrices, TestingMatricesPage, start)
	}

	_, tail, found := strings.Cut(rest, end)
	if !found {
		return "", fmt.Errorf("%w: %s has no %s", errTestingMatrices, TestingMatricesPage, end)
	}

	return head + start + "\n\n" + body + "\n" + end + tail, nil
}

func readMatrices(root string) (matrices, error) {
	demos, err := readMatrixDemos(root)
	if err != nil {
		return matrices{}, err
	}

	lanes, err := readMatrixLanes(root)
	if err != nil {
		return matrices{}, err
	}

	evidence, err := readMatrixEvidence(filepath.Join(root, TestingMatricesEvidence))
	if err != nil {
		return matrices{}, err
	}

	data := matrices{demos: demos, lanes: lanes, evidence: evidence}

	return data, data.knownSubjects()
}

// knownSubjects refuses an overlay mark of a demo that is not shipped or of a
// scenario no lane runs, which no cell would show.
func (data matrices) knownSubjects() error {
	known := map[string]bool{}
	for _, demo := range data.demos {
		known[demo.name] = true
	}

	for _, lane := range data.lanes {
		for _, scenario := range lane.scenarios {
			known[strings.TrimSuffix(strings.TrimPrefix(scenario, "^"), "$")] = true
		}
	}

	for _, key := range slices.Sorted(maps.Keys(data.evidence)) {
		if subject := strings.Split(key, "/")[1]; !known[subject] {
			return fmt.Errorf("%w: %s: %s is no shipped demo and no lane's scenario", errTestingMatrices, TestingMatricesEvidence, subject)
		}
	}

	return nil
}

func readMatrixDemos(root string) ([]matrixDemo, error) {
	entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(demosDir)))
	if err != nil {
		return nil, err
	}

	demos := []matrixDemo{}

	for _, entry := range entries {
		consumer := demoConsumerDir(root, entry.Name())
		if !entry.IsDir() || !isDir(consumer) {
			continue
		}

		demo, err := readMatrixDemo(entry.Name(), consumer)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", entry.Name(), err)
		}

		demos = append(demos, demo)
	}

	return demos, nil
}

func readMatrixDemo(name, consumer string) (matrixDemo, error) {
	demo := matrixDemo{name: name, hosted: map[string]bool{}, release: map[string]bool{}, admission: map[string]error{}}

	source, err := ServiceSnapshotSource(consumer)
	if err != nil {
		return demo, err
	}

	demo.source = source

	for provider, file := range map[string]string{campaignForgejo: forgejoReleaseWorkflow, campaignGitLab: GitLabConsumerBuildScript} {
		if demo.release[provider], err = exists(filepath.Join(consumer, filepath.FromSlash(file))); err != nil {
			return demo, err
		}

		demo.admission[provider] = matrixLifecycleAdmission(provider, consumer)
	}

	cfg, err := readMatrixDeclaration(filepath.Join(consumer, filepath.FromSlash(source)))
	if err != nil {
		return demo, err
	}

	// The artifacts, and so the ecosystem, are the default declaration's even
	// where the images are declared in a dedicated file.
	artifacts := cfg
	if source != GitLabDefaultArtifactsConfig {
		if artifacts, err = readMatrixDeclaration(filepath.Join(consumer, filepath.FromSlash(GitLabDefaultArtifactsConfig))); err != nil {
			return demo, err
		}
	}

	gpg, err := exists(filepath.Join(consumer, filepath.FromSlash(matrixGPGConfig)))
	if err != nil {
		return demo, err
	}

	if err := demo.readHosted(consumer, source); err != nil {
		return demo, err
	}

	return demo, demo.describeDeclaration(cfg, artifacts, gpg)
}

func readMatrixDeclaration(path string) (*config.Config, error) {
	body, err := os.ReadFile(path) //nolint:gosec // G304: a shipped demo's declaration.
	if err != nil {
		return nil, err
	}

	return parseServiceDeclaration(string(body))
}

// readHosted reads the demo's flows on hosted GitHub. An event has a line
// where the demo ships the fixture workflow the hosted settings name for it;
// the lifecycle has one where all three are shipped and the hosted settings
// accept the event. Its admission is the service snapshot every forge's
// lifecycle shares.
func (demo *matrixDemo) readHosted(consumer, source string) error {
	lifecycle := true

	for _, event := range githubEvents {
		settings, err := ParseGitHubTrialSettings(event, nil)
		if err != nil {
			return err
		}

		if demo.hosted[event], err = exists(filepath.Join(consumer, filepath.FromSlash(githubDemoWorkflows), settings.Workflow)); err != nil {
			return err
		}

		lifecycle = lifecycle && demo.hosted[event]
	}

	_, err := ParseGitHubTrialSettings(TrialLifecycle, nil)
	demo.hosted[TrialLifecycle] = lifecycle && err == nil
	demo.release[matrixGitHub] = demo.hosted[TrialRelease]

	_, demo.admission[matrixGitHub] = serviceSnapshotAdmission(consumer, source)

	return nil
}

// matrixLifecycleAdmission is the campaign's admission of a lifecycle of the
// demo on the provider, the GitLab release build included, as the lane plans
// it.
func matrixLifecycleAdmission(provider, consumer string) error {
	if err := campaignAdmitLifecycle(provider, consumer); err != nil {
		return err
	}

	if provider == campaignGitLab {
		return requireGitLabReleaseBuild(consumer)
	}

	return nil
}

// describeDeclaration reads the axis values the declarations carry: the
// images' from the snapshot source, the ecosystem from the artifacts.
func (demo *matrixDemo) describeDeclaration(cfg, artifacts *config.Config, gpg bool) error {
	if err := demo.readRoute(cfg); err != nil {
		return err
	}

	typed := pipeline.NewConfigPlan(cfg)
	demo.ecosystem = matrixEcosystem(pipeline.NewConfigPlan(artifacts).Artifacts.All)
	demo.images = matrixImages(typed)
	demo.signing = cmp.Or(string(cfg.Sign.Method), "default")

	if gpg {
		demo.signing += ", GPG declaration"
	}

	return nil
}

func matrixEcosystem(artifacts []pipeline.PlannedArtifact) string {
	families := make([]string, 0, len(artifacts))

	for _, artifact := range artifacts {
		family, err := snapshotimage.ToolchainFamily(artifact.ProjectType)
		if err != nil {
			family = string(artifact.ProjectType)
		}

		families = append(families, family)
	}

	slices.Sort(families)

	return cmp.Or(strings.Join(slices.Compact(families), ", "), "container only")
}

// readRoute reads the snapshot route the planner takes for the declaration's
// snapshot view: artifact-first when it plans producers for the images,
// container-first when the images build alone. An extracting image is named
// as such, and a snapshot the planner refuses otherwise takes no route and
// keeps the planner's refusal.
func (demo *matrixDemo) readRoute(cfg *config.Config) error {
	if len(cfg.Containers) == 0 {
		demo.route = routeNoContainer

		return nil
	}

	if slices.ContainsFunc(cfg.Containers, func(ctr config.Container) bool { return ctr.Extract != nil }) {
		demo.route = routeExtraction

		return nil
	}

	in, err := serviceSnapshotStandInInput()
	if err != nil {
		return err
	}

	plan, refusal := snapshotimage.New(pipeline.NewConfigPlan(snapshotView(cfg)), in)

	switch {
	case refusal != nil:
		demo.route, demo.snapshotRefusal = routeRefused, refusal
	case len(plan.Artifacts) > 0:
		demo.route = routeArtifactFirst
	default:
		demo.route = routeContainerFirst
	}

	return nil
}

func matrixImages(typed pipeline.ConfigPlan) string {
	for _, artifact := range typed.Artifacts.All {
		if artifact.WorkingDirectory != "." {
			return "nested artifacts (monorepo)"
		}
	}

	switch len(typed.Containers.All) {
	case 0:
		return "no image"
	case 1:
		return "one image"
	default:
		return fmt.Sprintf("%d images", len(typed.Containers.All))
	}
}

func readMatrixLanes(root string) ([]matrixLane, error) {
	files, err := filepath.Glob(filepath.Join(root, matrixCampaignsDir, "*.cases"))
	if err != nil {
		return nil, err
	}

	lanes := []matrixLane{}

	for _, file := range files {
		lane, err := readMatrixLane(file)
		if err != nil {
			return nil, err
		}

		lanes = append(lanes, lane)
	}

	return lanes, nil
}

func readMatrixLane(file string) (matrixLane, error) {
	name := strings.TrimSuffix(filepath.Base(file), ".cases")
	lane := matrixLane{file: name, provider: name[strings.LastIndex(name, "-")+1:], settings: map[string]string{}, consumers: map[string][]string{}}

	if !isCampaignProvider(lane.provider) {
		return lane, fmt.Errorf("%w: %s: a cases file is <lane>-<forgejo|gitlab>.cases", errTestingMatrices, file)
	}

	err := eachLine(file, func(_ int, line string) error {
		words := strings.Fields(line)
		if len(words) == 0 || strings.HasPrefix(words[0], "#") {
			return nil
		}

		switch {
		case words[0] == "setting" && len(words) == 2:
			key, value, _ := strings.Cut(words[1], "=")
			lane.settings[key] = value
		case words[0] == caseConsumer && len(words) >= 3:
			lane.consumers[words[1]] = append(lane.consumers[words[1]], words[2])
		case words[0] == settingScenario && len(words) >= 2:
			lane.scenarios = append(lane.scenarios, words[1])
		default:
			return fmt.Errorf("%w: %s: unreadable case %q", errTestingMatrices, file, line)
		}

		return nil
	})

	return lane, err
}

// readMatrixEvidence reads the overlay: `<provider> <demo|scenario> <event>
// <mark> [note]` a line, `#` comments and blank lines skipped.
func readMatrixEvidence(path string) (matrixEvidence, error) {
	evidence := matrixEvidence{}

	err := eachLine(path, func(number int, line string) error {
		words := strings.Fields(line)
		if len(words) == 0 || strings.HasPrefix(words[0], "#") {
			return nil
		}

		if len(words) < matrixEvidenceCols || !slices.Contains(matrixProviders, words[0]) {
			return fmt.Errorf("%w: %s:%d: want <forgejo|gitlab|github> <demo> <event> <mark> [note]", errTestingMatrices, path, number)
		}

		key := strings.Join(words[:3], "/")
		if _, dup := evidence[key]; dup {
			return fmt.Errorf("%w: %s:%d: %s is marked twice", errTestingMatrices, path, number, key)
		}

		evidence[key] = matrixMark{mark: words[3], note: strings.Join(words[4:], " ")}

		return nil
	})

	return evidence, err
}

func (data matrices) mark(provider, subject, event string) (matrixMark, bool) {
	mark, found := data.evidence[provider+"/"+subject+"/"+event]

	return mark, found
}

// cell writes a state with its subject and the overlay's note, if any.
func (mark matrixMark) cell(subject string) string {
	text := strings.TrimSpace(mark.mark + " " + subject)
	if mark.note != "" {
		text += " (" + mark.note + ")"
	}

	return text
}

// hasLine reports whether a lane of the provider runs the demo's event; on
// GitHub, whether the hosted harness runs it from the demo's own workflows.
func (data matrices) hasLine(provider string, demo matrixDemo, event string) bool {
	if provider == matrixGitHub {
		return demo.hosted[event]
	}

	for _, lane := range data.lanes {
		if lane.provider == provider && slices.Contains(lane.consumers[demo.name], event) {
			return true
		}
	}

	return false
}

// lifecycleState is a demo's lifecycle state on a provider: refused, the
// overlay's mark, a line that exists, or admitted without a line.
func (data matrices) lifecycleState(provider string, demo matrixDemo) matrixMark {
	if demo.admission[provider] != nil {
		return matrixMark{mark: markRefused}
	}

	if mark, found := data.mark(provider, demo.name, TrialLifecycle); found {
		return mark
	}

	if data.hasLine(provider, demo, TrialLifecycle) {
		return matrixMark{mark: markLine}
	}

	return matrixMark{mark: markNoLine, note: "admitted, no line"}
}

func (data matrices) releaseCell(provider string, demo matrixDemo) string {
	if !demo.release[provider] {
		return markNotApply
	}

	if mark, found := data.mark(provider, demo.name, TrialRelease); found {
		return mark.cell("")
	}

	if data.hasLine(provider, demo, TrialRelease) {
		return markLine
	}

	return markNoLine
}

// developmentCell is a demo's snapshot state on a provider: the admission's
// refusal (its reason is footnoted, and the overlay's note kept, as when a
// fixture of the demo ran), the overlay's mark, a lifecycle line that would
// run it, or admitted without one.
func (data matrices) developmentCell(provider string, demo matrixDemo) string {
	mark, found := data.mark(provider, demo.name, matrixDevelopment)

	if demo.admission[provider] != nil {
		return matrixMark{mark: markRefused, note: mark.note}.cell("")
	}

	if found {
		return mark.cell("")
	}

	if data.hasLine(provider, demo, TrialLifecycle) {
		return markLine + " lifecycle"
	}

	if data.hasLine(provider, demo, TrialDevRelease) {
		return markLine
	}

	return "admitted"
}

// gateCell is a demo's pull-request gate state on a provider.
func (data matrices) gateCell(provider string, demo matrixDemo) string {
	if mark, found := data.mark(provider, demo.name, TrialPullRequest); found {
		return mark.cell("")
	}

	if data.hasLine(provider, demo, TrialPullRequest) || data.hasLine(provider, demo, TrialLifecycle) {
		return markLine
	}

	return markNoLine
}

// matrixReason is the admission's refusal without the validation sentinel.
func matrixReason(err error) string {
	return strings.TrimPrefix(strings.TrimSuffix(err.Error(), ": "+errs.ErrValidation.Error()), TrialLifecycle+": ")
}

// markRank orders states, the best first: the overlay's live marks, then a
// line, then admitted without a line, then refused, then nothing.
func markRank(mark string) int {
	return slices.Index([]string{"✅", "◐", "⏳", "❌", markLine, markNoLine, markRefused, ""}, mark)
}

// bestOf is the best lifecycle state among the demos on the provider, with
// the demo it is of; no demo carrying the value is a gap of the demos.
func (data matrices) bestOf(provider string, demos []matrixDemo) string {
	if len(demos) == 0 {
		return markNoLine + " no demo"
	}

	best, subject := matrixMark{}, ""

	for _, demo := range demos {
		if state := data.lifecycleState(provider, demo); markRank(state.mark) < markRank(best.mark) {
			best, subject = state, demo.name
		}
	}

	return best.cell(subject)
}

// laneValueCell is the best lifecycle state of the provider's lifecycle lanes
// whose setting has the value, since cases-line axes are the lane's.
func (data matrices) laneValueCell(provider, setting, value string) string {
	best, subject := matrixMark{}, ""

	for _, lane := range data.lanes {
		if lane.provider != provider || lane.settings[setting] != value {
			continue
		}

		for _, demo := range data.demos {
			if state := data.lifecycleState(provider, demo); slices.Contains(lane.consumers[demo.name], TrialLifecycle) && markRank(state.mark) < markRank(best.mark) {
				best, subject = state, lane.file
			}
		}
	}

	if subject == "" {
		return markNoLine + " no lane"
	}

	return best.cell(subject)
}

type matrixRow struct {
	axis, value string
	cells       []string
}

func (data matrices) declaredRows(axis string, value func(matrixDemo) string) []matrixRow {
	byValue := map[string][]matrixDemo{}

	for _, demo := range data.demos {
		for v := range strings.SplitSeq(value(demo), ", ") {
			byValue[v] = append(byValue[v], demo)
		}
	}

	rows := make([]matrixRow, 0, len(byValue))

	for _, v := range slices.Sorted(maps.Keys(byValue)) {
		row := matrixRow{axis: axis, value: v}
		for _, provider := range matrixProviders {
			row.cells = append(row.cells, data.bestOf(provider, byValue[v]))
		}

		rows = append(rows, row)
	}

	return rows
}

func (data matrices) laneRows(axis, setting string, values []string) []matrixRow {
	rows := make([]matrixRow, 0, len(values))

	for _, value := range values {
		row := matrixRow{axis: axis, value: value}
		for _, provider := range matrixProviders {
			row.cells = append(row.cells, data.laneValueCell(provider, setting, value))
		}

		rows = append(rows, row)
	}

	return rows
}

// enforcementRows are one row per administrator check enforced by a lane,
// and one for none.
func (data matrices) enforcementRows() []matrixRow {
	rows := []matrixRow{}

	for _, check := range enforcement.Checks {
		rows = append(rows, data.laneRows("enforcement", "admin-"+string(check), []string{strconv.FormatBool(true)})[0])
		rows[len(rows)-1].value = "admin " + string(check)
	}

	none := data.laneRows("enforcement", "admin-"+string(enforcement.Checks[0]), []string{strconv.FormatBool(false)})[0]
	none.value = "none enforced"

	return append(rows, none)
}

func (data matrices) architectureRows() []matrixRow {
	amd64 := matrixRow{axis: "architecture", value: matrixArchRow}
	for _, provider := range matrixProviders {
		amd64.cells = append(amd64.cells, data.bestOf(provider, data.demos))
	}

	arm64 := matrixRow{axis: "architecture", value: "arm64"}
	for range matrixProviders {
		arm64.cells = append(arm64.cells, markNotApply+" D-5")
	}

	return []matrixRow{amd64, arm64}
}

// variantsSection is matrix 2: the lifecycle event over every axis value the
// demos and the lanes carry, then the kept scenario lines.
func (data matrices) variantsSection() string {
	var page strings.Builder

	page.WriteString("| Axis | Variant | Forgejo | GitLab | GitHub |\n|---|---|---|---|---|\n")

	rows := slices.Concat(
		data.declaredRows("ecosystem", func(demo matrixDemo) string { return demo.ecosystem }),
		data.declaredRows("route", func(demo matrixDemo) string { return demo.route }),
		data.declaredRows("images", func(demo matrixDemo) string { return demo.images }),
		data.declaredRows("signing", func(demo matrixDemo) string { return demo.signing }),
		data.laneRows("image builder", "image-builder", []string{ImageBuilderBuildah, ImageBuilderKaniko}),
		data.laneRows("visibility", "visibility", []string{"public", "private"}),
		data.enforcementRows(),
		data.architectureRows(),
	)

	axis := ""

	for _, row := range rows {
		shown := row.axis
		if shown == axis {
			shown = ""
		}

		axis = row.axis

		fmt.Fprintf(&page, "| %s | %s | %s |\n", shown, row.value, strings.Join(row.cells, " | "))
	}

	page.WriteString("\nKept scenario lines, by lane:\n\n| Lane | Scenario | State |\n|---|---|---|\n")

	for _, lane := range data.lanes {
		for _, scenario := range lane.scenarios {
			mark, found := data.mark(lane.provider, scenario, matrixScenario)
			if !found {
				mark = matrixMark{mark: markLine}
			}

			fmt.Fprintf(&page, "| %s | `%s` | %s |\n", lane.file, scenario, mark.cell(""))
		}
	}

	return page.String()
}

// projectKindsTable is matrix 1: every shipped demo as the project kind it
// embodies, read from its declaration, against the flows the orchestrators
// offer, a cell per forge. The snapshot column is the one the demo's route
// takes; the other is not applicable. The refusals' reasons follow the table.
func (data matrices) projectKindsTable() string {
	var page strings.Builder

	page.WriteString("| Demo (project kind) | PR / MR gate | Artifact-first snapshot | Container-first snapshot | Release | Full lifecycle |\n|---|---|---|---|---|---|\n")

	for _, demo := range data.demos {
		artifact, container := data.snapshotCells(demo)
		fmt.Fprintf(&page, "| %s (%s · %s · %s) | %s | %s | %s | %s | %s |\n", demo.name, demo.ecosystem, demo.route, demo.images,
			perForge(demo, data.gateCell), artifact, container,
			perForge(demo, data.releaseCell),
			perForge(demo, func(provider string, demo matrixDemo) string { return data.lifecycleState(provider, demo).cell("") }))
	}

	page.WriteString("\nRefused snapshots and lifecycles, with the planner's or the admission's reason:\n\n")

	for _, demo := range data.demos {
		if demo.snapshotRefusal != nil {
			fmt.Fprintf(&page, "- %s, every forge: the planner refuses its snapshot: %s\n", demo.name, demo.snapshotRefusal)
		}

		for _, provider := range matrixProviders {
			if err := demo.admission[provider]; err != nil && demo.route != routeNoContainer {
				fmt.Fprintf(&page, "- %s, %s: %s\n", demo.name, provider, matrixReason(err))
			}
		}
	}

	return page.String()
}

// perForge writes a flow's Forgejo, GitLab and GitHub cells.
func perForge(demo matrixDemo, cell func(string, matrixDemo) string) string {
	cells := make([]string, 0, len(matrixProviders))
	for _, provider := range matrixProviders {
		cells = append(cells, cell(provider, demo))
	}

	return strings.Join(cells, " / ")
}

// snapshotCells are the artifact-first and container-first snapshot cells:
// the demo's route decides which applies, and a demo without a container has
// neither.
func (data matrices) snapshotCells(demo matrixDemo) (string, string) {
	if demo.route == routeNoContainer {
		return markNotApply, markNotApply
	}

	state := perForge(demo, data.developmentCell)

	switch demo.route {
	case routeArtifactFirst:
		return state, markNotApply
	case routeContainerFirst:
		return markNotApply, state
	default:
		return state, state
	}
}
