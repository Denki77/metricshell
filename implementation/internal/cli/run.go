package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/Denki77/metricshell/implementation/internal/buildinfo"
	"github.com/Denki77/metricshell/implementation/internal/config"
	exitCodes "github.com/Denki77/metricshell/implementation/internal/constants"
	"github.com/Denki77/metricshell/implementation/internal/diagnostic"
	"github.com/Denki77/metricshell/implementation/internal/exposition"
	"github.com/Denki77/metricshell/implementation/internal/lifecycle"
	"github.com/Denki77/metricshell/implementation/internal/probe"
	"github.com/Denki77/metricshell/implementation/internal/selfmetric"
	"github.com/Denki77/metricshell/implementation/internal/shutdown"
	"github.com/Denki77/metricshell/implementation/internal/snapshot"
	"github.com/Denki77/metricshell/implementation/internal/workload"
)

const usage = `Usage:
  metricshell --version
  metricshell --help
  metricshell [options] -- executable [argument ...]

`

// Run executes the command line interface with the given arguments and returns the exit code.
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer, identity buildinfo.Info, now func() time.Time) int {
	logger := diagnostic.New(stderr, now)

	if len(args) == 1 {
		switch args[0] {
		case "--version":
			if _, err := fmt.Fprintln(stdout, identity.String()); err != nil {
				return exitCodes.ExitInternalFailure
			}
			return exitCodes.Success
		case "--help", "-h":
			if _, err := fmt.Fprint(stdout, usage); err != nil {
				return exitCodes.ExitInternalFailure
			}
			return exitCodes.Success
		}
	}
	if err := logger.WriteRuntimeInitializing(os.Getpid()); err != nil {
		return exitCodes.ExitInternalFailure
	}
	metrics, err := selfmetric.New(identity, selfmetric.FinalWaitImmediate, now)
	if err != nil {
		_ = logger.WriteRuntimeFailed()
		return exitCodes.ExitInternalFailure
	}
	machine, err := lifecycle.New(func(change lifecycle.Change) error {
		if err := metrics.SetRuntimeState(change.Current); err != nil {
			return err
		}
		return logger.WriteStateChanged(string(change.Previous), string(change.Current))
	})
	if err != nil {
		return exitCodes.ExitInternalFailure
	}

	configuration, err := config.Parse(args, now(), os.LookupEnv)
	if err == nil {
		holder := snapshot.NewHolder(snapshot.Zero())
		debugView := func() []byte {
			include, exclude := len(configuration.Exposition.Include), len(configuration.Exposition.Exclude)
			content, marshalErr := json.Marshal(map[string]any{
				"exposition_listen":      configuration.Exposition.Listen,
				"max_response_bytes":     configuration.Exposition.ResponseBytes,
				"max_concurrent_scrapes": configuration.Exposition.Concurrent,
				"metrics_include_count":  include,
				"metrics_exclude_count":  exclude,
			})
			if marshalErr != nil {
				return []byte("{}\n")
			}
			return append(content, '\n')
		}
		handler, handlerErr := exposition.NewHandler(configuration.Exposition, holder, metrics, probe.New(machine), debugView,
			func(outcome exposition.Outcome, status int) {
				_ = logger.WriteExpositionFailed(string(machine.State()), string(outcome), status)
			})
		if handlerErr != nil {
			return rejectConfiguration(machine, metrics, logger, handlerErr)
		}
		server, bindErr := exposition.Bind(configuration.Exposition, handler)
		if bindErr != nil {
			_ = metrics.AddCounter(selfmetric.RuntimeFailuresTotal, map[string]string{"reason": selfmetric.RuntimeFailureBind}, 1)
			_ = logger.WriteEndpointBindFailed("exposition", string(machine.State()))
			_ = machine.TransitionEvent(lifecycle.RuntimeFailed)
			_ = machine.TransitionEvent(lifecycle.CleanupCompleted)
			return exitCodes.ExitEndpointBindFailed
		}
		server.Start()
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), configuration.Exposition.WriteTimeout)
			defer cancel()
			_ = server.Shutdown(ctx)
		}()
		if err := logger.WriteEndpointBound("exposition", string(machine.State())); err != nil {
			return failLifecycle(machine, logger)
		}
		if err := machine.TransitionEvent(lifecycle.ConfigurationValidated); err != nil {
			return failLifecycle(machine, logger)
		}
		var shutdownPlan *shutdown.Plan
		result := workload.Run(configuration.Workload, stdin, stdout, stderr, configuration.Shutdown, workload.Observers{
			Started: func(pid, processGroupID int) error {
				if err := machine.TransitionEvent(lifecycle.WorkloadStarted); err != nil {
					return err
				}
				if err := metrics.SetWorkload(pid, true); err != nil {
					return err
				}
				if err := metrics.AddCounter(selfmetric.WorkloadStartsTotal, map[string]string{"outcome": selfmetric.WorkloadOutcomeStarted}, 1); err != nil {
					return err
				}
				return logger.WriteWorkloadStarted(pid, processGroupID)
			},
			PrimaryExited: func(exitCode int, forced bool) error {
				if err := machine.TransitionEvent(lifecycle.WorkloadExited); err != nil {
					return err
				}
				if err := metrics.SetWorkload(0, false); err != nil {
					return err
				}
				if err := metrics.SetWorkloadExitCode(exitCode); err != nil {
					return err
				}
				return logger.WriteWorkloadExited(exitCode, forced)
			},
			ChildReaped: func(kind string) error {
				if err := metrics.AddCounter(selfmetric.ChildrenReapedTotal, map[string]string{"kind": kind}, 1); err != nil {
					return err
				}
				return logger.WriteChildReaped(kind, string(machine.State()))
			},
			Failed: func() error {
				if machine.State() != lifecycle.Failed {
					if err := machine.TransitionEvent(lifecycle.RuntimeFailed); err != nil {
						return err
					}
				}
				if err := metrics.AddCounter(selfmetric.RuntimeFailuresTotal, map[string]string{"reason": selfmetric.RuntimeFailureInternal}, 1); err != nil {
					return err
				}
				return logger.WriteRuntimeFailed()
			},
			Shutdown: func(signal string, plan shutdown.Plan) error {
				shutdownPlan = &plan
				if err := machine.TransitionEvent(lifecycle.TerminationAfterSpawn); err != nil {
					return err
				}
				if err := metrics.SetGauge(selfmetric.ShutdownActive, nil, 1); err != nil {
					return err
				}
				if err := metrics.SetGauge(selfmetric.ShutdownDeadline, nil, float64(plan.Deadline.UnixNano())/float64(time.Second)); err != nil {
					return err
				}
				return logger.WriteShutdownStarted(signal, plan.Deadline, plan.Remaining(plan.StartedAt))
			},
			Forced: func(signal string, processGroupID int) error {
				if err := metrics.AddCounter(selfmetric.WorkloadForcedTotal, nil, 1); err != nil {
					return err
				}
				return logger.WriteShutdownForced(signal, processGroupID)
			},
			Signal: func(event workload.SignalEvent) error {
				switch event.Outcome {
				case workload.SignalForwarded:
					if err := metrics.AddCounter(selfmetric.WorkloadSignalsTotal, map[string]string{"signal": event.Name, "target": selfmetric.SignalTargetProcessGroup}, 1); err != nil {
						return err
					}
					return logger.WriteSignalForwarded(event.Name, string(machine.State()), event.ProcessGroupID)
				case workload.SignalIgnored:
					return logger.WriteSignalIgnored(event.Name, event.Reason, event.ProcessGroupID)
				default:
					return logger.WriteSignalFailed(event.Name, event.ProcessGroupID)
				}
			},
		})
		if shutdownPlan != nil && result.Started && machine.State() == lifecycle.Finalizing {
			duration := now().Sub(shutdownPlan.StartedAt)
			if err := metrics.Observe(selfmetric.ShutdownPhaseDuration, map[string]string{"phase": "total"}, duration.Seconds()); err != nil {
				return failLifecycle(machine, logger)
			}
			if err := logger.WriteShutdownCompleted(result.ExitCode, duration); err != nil {
				return failLifecycle(machine, logger)
			}
		}
		if result.StartFailed {
			if err := metrics.AddCounter(selfmetric.WorkloadStartsTotal, map[string]string{"outcome": selfmetric.WorkloadOutcomeStartFailed}, 1); err != nil {
				return failLifecycle(machine, logger)
			}
			if err := metrics.AddCounter(selfmetric.RuntimeFailuresTotal, map[string]string{"reason": selfmetric.RuntimeFailureWorkloadStart}, 1); err != nil {
				return failLifecycle(machine, logger)
			}
			if err := machine.TransitionEvent(lifecycle.WorkloadStartFailed); err != nil {
				return failLifecycle(machine, logger)
			}
			if writeErr := logger.WriteWorkloadStartFailed(); writeErr != nil {
				return exitCodes.ExitInternalFailure
			}
		}
		if result.Started && machine.State() == lifecycle.Finalizing {
			if err := machine.TransitionEvent(lifecycle.FinalizationImmediate); err != nil {
				return failLifecycle(machine, logger)
			}
		} else if !result.Started && !result.StartFailed && machine.State() == lifecycle.StartingWorkload {
			if err := machine.TransitionEvent(lifecycle.TerminationBeforeSpawn); err != nil {
				return failLifecycle(machine, logger)
			}
		}
		if machine.State() == lifecycle.Failed {
			_ = machine.TransitionEvent(lifecycle.CleanupCompleted)
		}
		return result.ExitCode
	}
	_ = metrics.AddCounter(selfmetric.RuntimeFailuresTotal, map[string]string{"reason": selfmetric.RuntimeFailureConfiguration}, 1)
	if writeErr := logger.WriteConfigurationRejected(err); writeErr != nil {
		_, err := fmt.Fprintln(stderr, `{"schema_version":"1","level":"error","event":"runtime.failed","component":"runtime","state":"initializing","message":"diagnostic write failed","reason":"internal","error_code":"INTERNAL_FAILURE"}`)
		if err != nil {
			fmt.Println(err.Error())
		}
		return exitCodes.ExitConfigurationRejected
	}
	if transitionErr := machine.TransitionEvent(lifecycle.InitializationFailed); transitionErr == nil {
		_ = machine.TransitionEvent(lifecycle.CleanupCompleted)
	}
	return exitCodes.ExitConfigurationInvalid
}

func rejectConfiguration(machine *lifecycle.Machine, metrics *selfmetric.Registry, logger *diagnostic.Logger, err error) int {
	_ = metrics.AddCounter(selfmetric.RuntimeFailuresTotal, map[string]string{"reason": selfmetric.RuntimeFailureConfiguration}, 1)
	_ = logger.WriteConfigurationRejected(err)
	if machine.State() == lifecycle.Initializing {
		_ = machine.TransitionEvent(lifecycle.InitializationFailed)
		_ = machine.TransitionEvent(lifecycle.CleanupCompleted)
	}
	return exitCodes.ExitConfigurationInvalid
}

func failLifecycle(machine *lifecycle.Machine, logger *diagnostic.Logger) int {
	if machine.State() != lifecycle.Failed {
		_ = machine.TransitionEvent(lifecycle.RuntimeFailed)
	}
	_ = logger.WriteRuntimeFailed()
	_ = machine.TransitionEvent(lifecycle.CleanupCompleted)
	return exitCodes.ExitInternalFailure
}
