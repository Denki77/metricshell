# Contributing

Thank you for taking an interest in MetricShell.

## Development model

- Production code lives in `implementation/`.
- Architecture evidence lives in `research/` and must not become a production dependency.
- English and Russian documentation should be updated together when user-facing behavior changes.
- Build and test commands are Docker-only; a host Go toolchain is not part of the supported workflow.

## Before opening a pull request

Run:

```sh
cd implementation
make ci
```

For focused implementation work, run the relevant wave target as well:

```sh
make wave6
```

## Pull requests

Please include:

- what changed;
- which requirement, issue or ADR it implements;
- which Docker checks were run;
- any known limitations or follow-up work.
