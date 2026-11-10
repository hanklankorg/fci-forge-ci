// SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government
// SPDX-License-Identifier: EUPL-1.2 OR GPL-3.0-or-later

package livetest

import "github.com/diggsweden/forge-ci/internal/domain/enforcement"

// campaignAdminSetting is a check's administrator setting as the testing
// matrices and the evidence overlay name it, admin-<check>.
func campaignAdminSetting(check enforcement.Check) string { return "admin-" + string(check) }
