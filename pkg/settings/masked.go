// Copyright 2019 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package settings

import "context"

// MaskedSetting is a wrapper for non-reportable settings that were retrieved
// for reporting (see SetReportable and LookupForReportingByKey).
type MaskedSetting struct {
	setting NonMaskedSetting
	// forReporting is set when the setting was retrieved for reporting. Its
	// value is then always hidden, even for a sensitive setting.
	forReporting bool
}

var _ Setting = &MaskedSetting{}

// redactSensitiveSettingsEnabled enables or disables the redaction of sensitive
// cluster settings.
var redactSensitiveSettingsEnabled = RegisterBoolSetting(
	ApplicationLevel,
	"server.redact_sensitive_settings.enabled",
	"enables or disables the redaction of sensitive settings in the output of SHOW CLUSTER SETTINGS and "+
		"SHOW ALL CLUSTER SETTINGS for users without the MODIFYCLUSTERSETTING privilege",
	false,
	WithPublic,
)

// String hides the underlying value.
func (s *MaskedSetting) String(sv *Values) string {
	// Special case for non-reportable/sensitive strings: we still want
	// to distinguish empty from non-empty (= customized).
	if st, ok := s.setting.(*StringSetting); ok && st.String(sv) == "" {
		return ""
	}
	isSensitive := false
	if st, ok := s.setting.(internalSetting); ok {
		isSensitive = st.isSensitive()
	}
	sensitiveRedactionEnabled := redactSensitiveSettingsEnabled.Get(sv)
	// Non-reportable settings are always redacted. Sensitive settings are
	// redacted if the redaction cluster setting is enabled, and always when
	// they are reported.
	if !isSensitive || sensitiveRedactionEnabled || s.forReporting {
		return "<redacted>"
	}
	return s.setting.String(sv)
}

// DefaultString returns the default value for the setting as a string.
func (s *MaskedSetting) DefaultString() (string, error) {
	return s.setting.DecodeToString(s.setting.EncodedDefault())
}

// Visibility returns the visibility setting for the underlying setting.
func (s *MaskedSetting) Visibility() Visibility {
	return s.setting.Visibility()
}

// InternalKey returns the key string for the underlying setting.
func (s *MaskedSetting) InternalKey() InternalKey {
	return s.setting.InternalKey()
}

// Name returns the name string for the underlying setting.
func (s *MaskedSetting) Name() SettingName {
	return s.setting.Name()
}

// Description returns the description string for the underlying setting.
func (s *MaskedSetting) Description() string {
	return s.setting.Description()
}

// Typ returns the short (1 char) string denoting the type of setting.
func (s *MaskedSetting) Typ() string {
	return s.setting.Typ()
}

// Class returns the class for the underlying setting.
func (s *MaskedSetting) Class() Class {
	return s.setting.Class()
}

// ValueOrigin returns the origin of the current value of the setting.
func (s *MaskedSetting) ValueOrigin(ctx context.Context, sv *Values) ValueOrigin {
	return s.setting.ValueOrigin(ctx, sv)
}

// IsUnsafe returns whether the underlying setting is unsafe.
func (s *MaskedSetting) IsUnsafe() bool {
	return s.setting.IsUnsafe()
}

// TestingIsReportable is used in testing for reportability.
func TestingIsReportable(s Setting) bool {
	if _, ok := s.(*MaskedSetting); ok {
		return false
	}
	if e, ok := s.(internalSetting); ok {
		return e.isReportable()
	}
	return true
}
