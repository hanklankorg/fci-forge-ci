// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package config_test

import (
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appconfig "github.com/diggsweden/forge-ci/internal/app/config"
	"github.com/diggsweden/forge-ci/internal/domain/errs"
	"github.com/diggsweden/forge-ci/internal/domain/output"
	"github.com/diggsweden/forge-ci/internal/domain/projecttype"
)

const releaseStageDeclaration = `artifacts:
  - name: backend
    project-type: maven
    working-directory: backend
  - name: frontend
    project-type: npm
    working-directory: frontend
`

// The GitLab release plans what it bumps and builds from the declaration;
// without one, and only for the default path, it plans neither.
func TestLoadReleaseStagePlans_PlansTheDeclarationOrNothing(t *testing.T) {
	t.Parallel()

	declared := fstest.MapFS{".forge-ci/artifacts.yml": {Data: []byte(releaseStageDeclaration)}}

	plans, err := appconfig.LoadReleaseStagePlans(appconfig.EmitConfigPlanInput{Path: ".forge-ci/artifacts.yml", FS: declared}, output.Annotator{}, false)
	require.NoError(t, err)
	assert.True(t, plans.Prepare.Targets.VersionBump.Runs)
	require.Len(t, plans.Prepare.Targets.VersionBump.Items, 2)
	assert.Equal(t, projecttype.Maven, plans.Prepare.Targets.VersionBump.Items[0].ProjectType)
	assert.True(t, plans.Build.Targets.Maven.Runs)
	assert.True(t, plans.Build.Targets.NPM.Runs)

	absent := appconfig.EmitConfigPlanInput{Path: ".forge-ci/artifacts.yml", FS: fstest.MapFS{}}

	plans, err = appconfig.LoadReleaseStagePlans(absent, output.Annotator{}, true)
	require.NoError(t, err)
	assert.Empty(t, plans.Prepare.Targets.VersionBump.Items)
	assert.False(t, plans.Build.Targets.Maven.Runs)

	_, err = appconfig.LoadReleaseStagePlans(absent, output.Annotator{}, false)
	require.ErrorIs(t, err, errs.ErrMissingInput)
}
