package cli

// EnvMapRunner returns middleware that resolves environment variables
// for flags and options not provided on the command line. The mapping
// keys are flag/option names and values are environment variable names.
//
// The middleware inspects the call before [Call.ApplyDefaults], so
// [OptionSet.Has] and [FlagSet.Has] report only CLI-provided values.
// For options, the environment variable value is set directly.
// For boolean flags, any non-empty environment variable value sets
// the flag to true.
//
//	mw := cli.EnvMapRunner(map[string]string{
//		"host":    "API_HOST",
//		"verbose": "VERBOSE",
//	}, os.LookupEnv)
func EnvMapRunner(mapping map[string]string, lookupEnv LookupFunc) func(Runner) Runner {
	return func(next Runner) Runner {
		return RunnerFunc(func(out *Output, call *Call) error {
			for name, envVar := range mapping {
				val, ok := lookupEnv(envVar)
				if !ok {
					continue
				}
				if !call.Flags.Has(name) && !call.Options.Has(name) {
					if call.flagDefaults != nil {
						if _, isFlag := call.flagDefaults[name]; isFlag {
							call.Flags[name] = val != ""
							continue
						}
					}
					call.Options.Set(name, val)
				}
			}
			return next.RunCLI(out, call)
		})
	}
}
