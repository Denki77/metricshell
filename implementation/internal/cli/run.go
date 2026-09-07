package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/Denki77/metricshell/implementation/internal/buildinfo"
	"github.com/Denki77/metricshell/implementation/internal/config"
	exitCodes "github.com/Denki77/metricshell/implementation/internal/constants"
	"github.com/Denki77/metricshell/implementation/internal/diagnostic"
	"github.com/Denki77/metricshell/implementation/internal/exposition"
	"github.com/Denki77/metricshell/implementation/internal/fileingest"
	"github.com/Denki77/metricshell/implementation/internal/finalwait"
	"github.com/Denki77/metricshell/implementation/internal/httpingest"
	"github.com/Denki77/metricshell/implementation/internal/ingestion"
	"github.com/Denki77/metricshell/implementation/internal/lifecycle"
	"github.com/Denki77/metricshell/implementation/internal/probe"
	"github.com/Denki77/metricshell/implementation/internal/selfmetric"
	"github.com/Denki77/metricshell/implementation/internal/shutdown"
	"github.com/Denki77/metricshell/implementation/internal/snapshot"
	"github.com/Denki77/metricshell/implementation/internal/socketingest"
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
		if err := metrics.SetFinalWaitMode(selfmetric.FinalWaitMode(configuration.FinalWait.Mode)); err != nil {
			return failLifecycle(machine, logger)
		}
		if err := metrics.SetGauge(selfmetric.FinalWaitRequiredScrapes, nil, float64(configuration.FinalWait.RequiredScrapes)); err != nil {
			return failLifecycle(machine, logger)
		}
		holder := snapshot.NewHolder(snapshot.Zero())
		metricsObserver, observerErr := ingestion.NewMetricsObserver(metrics, now)
		if observerErr != nil {
			return failLifecycle(machine, logger)
		}
		diagnosticObserver, observerErr := ingestion.NewDiagnosticObserver(logger, func() string { return string(machine.State()) })
		if observerErr != nil {
			return failLifecycle(machine, logger)
		}
		core, coreErr := ingestion.New(holder, configuration.Limits, configuration.ConcurrentIngestion, configuration.PendingIngestion,
			ingestion.MultiObserver{metricsObserver, diagnosticObserver})
		if coreErr != nil {
			return rejectConfiguration(machine, metrics, logger, coreErr)
		}
		debugView := func() []byte {
			include, exclude := len(configuration.Exposition.Include), len(configuration.Exposition.Exclude)
			content, marshalErr := json.Marshal(map[string]any{
				"exposition_listen":      configuration.Exposition.Listen,
				"max_response_bytes":     configuration.Exposition.ResponseBytes,
				"max_concurrent_scrapes": configuration.Exposition.Concurrent,
				"metrics_include_count":  include,
				"metrics_exclude_count":  exclude,
				"final_wait_mode":        configuration.FinalWait.Mode,
				"final_wait_duration":    configuration.FinalWait.Duration.String(),
				"final_wait_timeout":     configuration.FinalWait.Timeout.String(),
				"final_wait_required":    configuration.FinalWait.RequiredScrapes,
				"final_wait_grace":       configuration.FinalWait.CompletionGrace.String(),
				"ingestion_transport":    configuration.IngestionTransport,
				"unix_socket_path":       configuration.UnixSocketPath,
				"http_ingestion_listen":  configuration.HTTPIngestionListen,
				"snapshot_file_path":     configuration.File.Path,
				"snapshot_bytes":         configuration.Limits.SnapshotBytes,
				"decoded_input_bytes":    configuration.Limits.DecodedBytes,
				"concurrent_ingestions":  configuration.ConcurrentIngestion,
				"pending_ingestions":     configuration.PendingIngestion,
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
		ingestionContext, stopIngestion := context.WithCancel(context.Background())
		stopSelectedIngestion, ingestionErr := startIngestion(ingestionContext, configuration, core)
		if ingestionErr != nil {
			stopIngestion()
			_ = server.Close()
			_ = metrics.AddCounter(selfmetric.RuntimeFailuresTotal, map[string]string{"reason": selfmetric.RuntimeFailureBind}, 1)
			_ = logger.WriteEndpointBindFailed("ingestion", string(machine.State()))
			_ = machine.TransitionEvent(lifecycle.RuntimeFailed)
			_ = machine.TransitionEvent(lifecycle.CleanupCompleted)
			return exitCodes.ExitEndpointBindFailed
		}
		defer func() {
			stopIngestion()
			_ = stopSelectedIngestion()
		}()
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), configuration.Exposition.WriteTimeout)
			defer cancel()
			_ = server.Shutdown(ctx)
		}()
		if err := logger.WriteEndpointBound("exposition", string(machine.State())); err != nil {
			return failLifecycle(machine, logger)
		}
		if err := logger.WriteEndpointBound("ingestion", string(machine.State())); err != nil {
			return failLifecycle(machine, logger)
		}
		if err := machine.TransitionEvent(lifecycle.ConfigurationValidated); err != nil {
			return failLifecycle(machine, logger)
		}
		terminationContext, stopTermination := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
		defer stopTermination()
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
			freezeContext, cancelFreeze := finalizationContext(configuration, shutdownPlan, now())
			final := core.CloseAndFreeze(freezeContext)
			cancelFreeze()
			metrics.SetActiveSnapshot(final)
			if shutdownPlan != nil {
				if err := machine.TransitionEvent(lifecycle.FinalizationImmediate); err != nil {
					return failLifecycle(machine, logger)
				}
			} else {
				waiter, waitErr := finalwait.New(configuration.FinalWait, final.Generation())
				if waitErr != nil {
					return failLifecycle(machine, logger)
				}
				waitStartedAt := now()
				if configuration.FinalWait.Mode == finalwait.Immediate {
					if err := startFinalWaitObservability(metrics, logger, configuration.FinalWait, lifecycle.Finalizing, waitStartedAt); err != nil {
						return failLifecycle(machine, logger)
					}
					var waitResult finalwait.Result
					if waitResult, waitErr = waiter.Wait(terminationContext); waitErr != nil {
						_ = completeFinalWaitObservability(metrics, logger, finalwait.Result{Reason: finalwait.ReasonRuntimeFailure, Generation: final.Generation()}, lifecycle.Finalizing, now().Sub(waitStartedAt))
						return failLifecycle(machine, logger)
					}
					if err := completeFinalWaitObservability(metrics, logger, waitResult, lifecycle.Finalizing, now().Sub(waitStartedAt)); err != nil {
						return failLifecycle(machine, logger)
					}
					if err := machine.TransitionEvent(lifecycle.FinalizationImmediate); err != nil {
						return failLifecycle(machine, logger)
					}
				} else {
					handler.SetFinalWait(waiter, func(response exposition.FinalResponse) {
						observeFinalResponse(metrics, logger, response)
					})
					var waitResult finalwait.Result
					waitResult, waitErr = waiter.WaitTransition(terminationContext, func() error {
						if err := machine.TransitionEvent(lifecycle.FinalizationWait); err != nil {
							return err
						}
						waitStartedAt = now()
						return startFinalWaitObservability(metrics, logger, configuration.FinalWait, lifecycle.FinalWait, waitStartedAt)
					})
					if waitErr != nil {
						_ = completeFinalWaitObservability(metrics, logger, finalwait.Result{Reason: finalwait.ReasonRuntimeFailure, Generation: final.Generation()}, machine.State(), now().Sub(waitStartedAt))
						return failLifecycle(machine, logger)
					}
					if err := completeFinalWaitObservability(metrics, logger, waitResult, lifecycle.FinalWait, now().Sub(waitStartedAt)); err != nil {
						return failLifecycle(machine, logger)
					}
					if waitResult.Reason == finalwait.ReasonRequiredScrapes {
						drainContext, cancelDrain := context.WithTimeout(context.Background(), configuration.FinalWait.CompletionGrace)
						drained := handler.Drain(drainContext)
						if !drained || server.Shutdown(drainContext) != nil {
							_ = server.Close()
						}
						cancelDrain()
					} else if waitResult.Reason == finalwait.ReasonExternalTermination {
						_ = server.Close()
					}
					if err := machine.TransitionEvent(lifecycle.FinalWaitCompleted); err != nil {
						return failLifecycle(machine, logger)
					}
				}
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

func startFinalWaitObservability(metrics *selfmetric.Registry, logger *diagnostic.Logger, configuration finalwait.Config, state lifecycle.State, startedAt time.Time) error {
	if configuration.Mode != finalwait.Immediate {
		if err := metrics.SetGauge(selfmetric.FinalWaitActive, nil, 1); err != nil {
			return err
		}
	}
	if err := metrics.SetGauge(selfmetric.FinalWaitCompletedScrapes, nil, 0); err != nil {
		return err
	}
	deadline := finalWaitDeadline(configuration, startedAt)
	if !deadline.IsZero() {
		if err := metrics.SetGauge(selfmetric.FinalWaitDeadline, nil, float64(deadline.UnixNano())/float64(time.Second)); err != nil {
			return err
		}
	}
	return logger.WriteFinalWaitStarted(string(configuration.Mode), string(state), deadline)
}

func observeFinalResponse(metrics *selfmetric.Registry, logger *diagnostic.Logger, response exposition.FinalResponse) {
	_ = metrics.AddCounter(selfmetric.FinalScrapeAttemptsTotal, map[string]string{"outcome": string(response.Outcome)}, 1)
	if response.Counted {
		_ = metrics.SetGauge(selfmetric.FinalWaitCompletedScrapes, nil, float64(response.Completed))
		_ = logger.WriteFinalScrapeCounted(response.RequestID, response.Generation)
		return
	}
	_ = logger.WriteFinalScrapeNotCounted(response.RequestID, string(response.Outcome))
}

func completeFinalWaitObservability(metrics *selfmetric.Registry, logger *diagnostic.Logger, result finalwait.Result, state lifecycle.State, duration time.Duration) error {
	if err := metrics.SetGauge(selfmetric.FinalWaitActive, nil, 0); err != nil {
		return err
	}
	if err := metrics.SetGauge(selfmetric.FinalWaitDeadline, nil, 0); err != nil {
		return err
	}
	if err := metrics.SetGauge(selfmetric.FinalWaitCompletedScrapes, nil, float64(result.Completed)); err != nil {
		return err
	}
	if err := metrics.AddCounter(selfmetric.FinalWaitCompletionsTotal, map[string]string{"reason": string(result.Reason)}, 1); err != nil {
		return err
	}
	return logger.WriteFinalWaitCompleted(string(result.Reason), string(state), duration)
}

func finalWaitDeadline(configuration finalwait.Config, startedAt time.Time) time.Time {
	switch configuration.Mode {
	case finalwait.Duration:
		return startedAt.Add(configuration.Duration)
	case finalwait.Scrapes:
		return startedAt.Add(configuration.Timeout)
	default:
		return time.Time{}
	}
}

func startIngestion(ctx context.Context, configuration config.Config, core *ingestion.Core) (func() error, error) {
	switch ingestion.Transport(configuration.IngestionTransport) {
	case ingestion.File:
		fileConfiguration := fileingest.Config{
			Path: configuration.File.Path, ReconcileInterval: configuration.File.ReconcileInterval,
			DecodedBytes: configuration.File.DecodedBytes,
		}
		if err := ensurePrivateParent(configuration.File.Path); err != nil {
			return nil, err
		}
		reconciler, err := fileingest.New(fileConfiguration, core, nil)
		if err != nil {
			return nil, err
		}
		go func() { _ = reconciler.Run(ctx) }()
		return func() error { return nil }, nil
	case ingestion.Unix:
		socketConfiguration := socketingest.Config{
			FrameBytes: configuration.Socket.FrameBytes, Parts: configuration.Socket.Parts,
			Connections: configuration.Socket.Connections, Transactions: configuration.Socket.Transactions,
			TransactionTimeout: configuration.Socket.TransactionTimeout, ReadTimeout: configuration.Socket.ReadTimeout,
			WriteTimeout: configuration.Socket.WriteTimeout, DecodedBytes: configuration.Socket.DecodedBytes,
			SnapshotBytes: configuration.Socket.SnapshotBytes,
		}
		if err := ensurePrivateParent(configuration.UnixSocketPath); err != nil {
			return nil, err
		}
		server, err := socketingest.Listen(configuration.UnixSocketPath, socketConfiguration, core, nil)
		if err != nil {
			return nil, err
		}
		go func() { _ = server.Serve(ctx) }()
		return server.Close, nil
	case ingestion.HTTP:
		httpConfiguration := httpingest.Config{
			WireBytes: configuration.HTTPIngestion.WireBytes, DecodedBytes: configuration.HTTPIngestion.DecodedBytes,
			ReadHeaderTimeout: configuration.HTTPIngestion.ReadHeaderTimeout, ReadTimeout: configuration.HTTPIngestion.ReadTimeout,
			WriteTimeout: configuration.HTTPIngestion.WriteTimeout, IdleTimeout: configuration.HTTPIngestion.IdleTimeout,
			MaxHeaderBytes: configuration.HTTPIngestion.MaxHeaderBytes,
		}
		handler, err := httpingest.NewHandler(httpConfiguration, core)
		if err != nil {
			return nil, err
		}
		server, err := httpingest.NewServer(configuration.HTTPIngestionListen, handler, httpConfiguration)
		if err != nil {
			return nil, err
		}
		listener, err := net.Listen("tcp", configuration.HTTPIngestionListen)
		if err != nil {
			return nil, err
		}
		go func() {
			err := server.Serve(listener)
			if err != nil && err != http.ErrServerClosed {
				_ = server.Close()
			}
		}()
		return func() error {
			shutdownContext, cancel := context.WithTimeout(context.Background(), configuration.HTTPIngestion.WriteTimeout)
			defer cancel()
			return server.Shutdown(shutdownContext)
		}, nil
	default:
		return nil, fmt.Errorf("unknown ingestion transport %q", configuration.IngestionTransport)
	}
}

func ensurePrivateParent(path string) error {
	if !filepath.IsAbs(path) {
		return fmt.Errorf("path must be absolute")
	}
	return os.MkdirAll(filepath.Dir(path), 0o700)
}

func finalizationContext(configuration config.Config, plan *shutdown.Plan, at time.Time) (context.Context, context.CancelFunc) {
	if plan != nil {
		ctx, cancel, err := plan.PhaseContext(context.Background(), shutdown.Finalization, plan.Reserve, at)
		if err == nil {
			return ctx, cancel
		}
	}
	return context.WithTimeout(context.Background(), configuration.FinalWait.Timeout)
}
