// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package cliflagcfg

import (
	"github.com/cockroachdb/cockroach/pkg/cli/cliflags"
	"github.com/cockroachdb/cockroach/pkg/util/envutil"
	"github.com/cockroachdb/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const (
	// envValueAnnotationKey is the map key used in pflag.Flag instances
	// to associate flags with a possible default value set by an
	// env var.
	envValueAnnotationKey = "envvalue"
)

// registerEnvVarDefault registers a deferred initialization of a flag
// from an environment variable.
// The caller is responsible for ensuring that the flagInfo has been
// defined in the FlagSet already.
func registerEnvVarDefault(f *pflag.FlagSet, flagInfo cliflags.FlagInfo, depth int) {
	if flagInfo.EnvVar == "" {
		return
	}

	value, set := envutil.EnvString(flagInfo.EnvVar, depth+1)
	if !set {
		// Env var is not set. Nothing to do.
		return
	}

	if err := f.SetAnnotation(flagInfo.Name, envValueAnnotationKey, []string{flagInfo.EnvVar, value}); err != nil {
		// This should never happen: an error is only returned if the flag
		// name was not defined yet.
		panic(err)
	}
}

// ProcessEnvVarDefaults injects the current value of flag-related
// environment variables into the initial value of the settings linked
// to the flags, during initialization and before the command line is
// actually parsed. For example, it will inject the value of
// $COCKROACH_URL into the urlParser object linked to the --url flag.
func ProcessEnvVarDefaults(cmd *cobra.Command) error {
	fl := FlagSetForCmd(cmd)

	var retErr error
	fl.VisitAll(func(f *pflag.Flag) {
		envv, ok := f.Annotations[envValueAnnotationKey]
		if !ok || len(envv) < 2 {
			// No env var associated. Nothing to do.
			return
		}
		varName, value := envv[0], envv[1]
		if err := fl.Set(f.Name, value); err != nil {
			retErr = errors.CombineErrors(retErr,
				errors.Wrapf(err, "setting --%s from %s", f.Name, varName))
		}
	})
	return retErr
}
