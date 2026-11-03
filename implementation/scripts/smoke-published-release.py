#!/usr/bin/env python3
"""Anonymous consumer test: published v0.2.2 images only; no builds or login.

Requires Python 3, Docker and a daemon supporting both Linux architectures
(native or QEMU). Each command and the complete run have finite deadlines.
"""
import argparse
import hashlib
import http.client
import json
import os
import signal
from pathlib import Path
import subprocess
import tempfile
import time
import urllib.error
import urllib.request

VERSION = '0.2.2'
REVISION = '4b19f7e20b2aab366c5cb69edd9e831bc7c89d58'
RUNTIME = 'sha256:20d5b0cec5195d677f49b65bf315637bf0fed6899a936de527392f85df57f8ec'
EXAMPLE = 'sha256:56fd77bb5534549a12b1e1a7948a3cd2d63a08de4ee79bb32bbbb8b2a13e0f6b'
IMAGES = {'metricshell': RUNTIME, 'metricshell-artifact': RUNTIME, 'metricshell-example': EXAMPLE}


def fetch(url, headers=None):
    with urllib.request.urlopen(urllib.request.Request(url, headers=headers or {}), timeout=30) as response:
        return response.read()


def anonymous_manifest(package, expected):
    # This is an anonymous registry bearer token, never a user/CI credential.
    token = json.loads(fetch('https://ghcr.io/token?service=ghcr.io&scope=repository:denki77/' + package + ':pull'))['token']
    raw = fetch('https://ghcr.io/v2/denki77/' + package + '/manifests/' + VERSION, {
        'Authorization': 'Bearer ' + token,
        'Accept': 'application/vnd.oci.image.index.v1+json, application/vnd.docker.distribution.manifest.list.v2+json'})
    actual = 'sha256:' + hashlib.sha256(raw).hexdigest()
    if actual != expected:
        raise RuntimeError(package + ': digest mismatch: ' + actual)
    platforms = {(m.get('platform', {}).get('os'), m.get('platform', {}).get('architecture'))
                 for m in json.loads(raw).get('manifests', [])}
    if not {('linux', 'amd64'), ('linux', 'arm64')} <= platforms:
        raise RuntimeError(package + ': missing Linux architecture')
    print('ANONYMOUS MANIFEST PASS:', package, actual, 'linux/amd64 linux/arm64', flush=True)
    return {m['platform']['architecture']: m['digest'] for m in json.loads(raw)['manifests']
            if m.get('platform', {}).get('os') == 'linux'}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--platform', choices=['amd64', 'arm64', 'all'], default='all')
    parser.add_argument('--timeout', type=int, default=900)
    args = parser.parse_args()
    if args.timeout <= 0:
        parser.error('--timeout must be positive')
    def interrupted(signum, frame):
        raise RuntimeError('smoke test interrupted or total timeout exceeded')

    signal.signal(signal.SIGTERM, interrupted)
    signal.signal(signal.SIGINT, interrupted)
    signal.signal(signal.SIGALRM, interrupted)
    signal.alarm(args.timeout)
    deadline = time.monotonic() + args.timeout
    failures = []
    manifests = {}
    for package, digest in IMAGES.items():
        try:
            manifests[package] = anonymous_manifest(package, digest)
        except (urllib.error.URLError, ValueError, RuntimeError) as error:
            failures.append(package)
            print('ANONYMOUS MANIFEST FAIL:', package, str(error), flush=True)
            print('Owner action if package is private: GitHub profile > Packages > ' + package +
                  ' > Package settings > Change visibility > Public. Do not retry with login.', flush=True)
    if failures:
        raise RuntimeError('anonymous access/digest/platform verification failed: ' + ', '.join(failures))

    # Read only the daemon address before isolating configuration. No auth config is copied.
    endpoint = os.environ.get('DOCKER_HOST')
    if not endpoint:
        context = subprocess.run(['docker', 'context', 'inspect', '--format', '{{.Endpoints.docker.Host}}'],
                                 check=True, text=True, capture_output=True, timeout=15)
        endpoint = context.stdout.strip()
    if not endpoint.startswith(('unix://', 'npipe://')):
        raise RuntimeError('use a local Docker daemon; remote TLS credentials are not imported')
    containers = []
    with tempfile.TemporaryDirectory(prefix='metricshell-anonymous-') as temporary:
        config = Path(temporary) / 'config'
        config.mkdir(mode=0o700)
        (config / 'config.json').write_text('{"auths":{}}\n')
        env = {k: v for k, v in os.environ.items() if k not in
               ('DOCKER_AUTH_CONFIG', 'DOCKER_CONTEXT', 'REGISTRY_AUTH_FILE', 'DOCKER_CERT_PATH', 'DOCKER_TLS_VERIFY')}
        env['DOCKER_CONFIG'] = str(config)
        docker = ['docker', '--config', str(config), '--host', endpoint]

        def run(*arguments, timeout=120, check=True):
            remaining = deadline - time.monotonic()
            if remaining <= 0:
                raise RuntimeError('smoke test total timeout exceeded')
            result = subprocess.run(docker + list(arguments), env=env, text=True,
                                    stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                                    timeout=min(timeout, remaining), check=False)
            if check and result.returncode:
                raise RuntimeError('docker ' + arguments[0] + ': ' + result.stderr.strip())
            return result

        def create(*arguments):
            # Assign name before starting so cleanup also covers an interrupted create/start.
            name = 'metricshell-smoke-' + str(os.getpid()) + '-' + str(len(containers))
            containers.append(name)
            run('create', '--name', name, *arguments)
            run('start', name)
            return name

        def wait(name, expected):
            result = run('wait', name, timeout=40)
            if result.stdout.strip() != str(expected):
                raise RuntimeError(name + ': wrong exit code: ' + result.stdout.strip())

        try:
            run('info')
            for arch in (['amd64', 'arm64'] if args.platform == 'all' else [args.platform]):
                platform = 'linux/' + arch
                for package, digest in IMAGES.items():
                    run('pull', '--platform', platform, 'ghcr.io/denki77/' + package + ':' + VERSION, timeout=180)
                    print('ANONYMOUS PULL PASS:', package, platform, flush=True)
                # Pin platform children from the digest-verified public index. This also works
                # with classic Docker stores that cannot retain both children under one index.
                refs = {package: 'ghcr.io/denki77/' + package + '@' + manifests[package][arch]
                        for package in IMAGES}
                for reference in refs.values():
                    run('pull', '--platform', platform, reference, timeout=180)
                runtime = refs['metricshell']
                example = refs['metricshell-example']
                for image, entry in [(runtime, '/metricshell'),
                                     (refs['metricshell-artifact'], '/metricshell'),
                                     (example, '/usr/local/bin/metricshell')]:
                    name = create('--platform', platform, '--entrypoint', entry, image, '--version')
                    wait(name, 0)
                    actual = run('logs', name).stdout.strip()
                    if actual != 'metricshell version=' + VERSION + ' revision=' + REVISION:
                        raise RuntimeError('version/revision mismatch: ' + actual)
                name = create('--platform', platform, '-e', 'METRICSHELL_FINAL_WAIT_MODE=immediate',
                              runtime, '--ingestion-transport=http', '--http-ingestion-listen=127.0.0.1:9091',
                              '--', '/metricshell', '--version')
                wait(name, 0)
                # A published nested supervisor is a finite workload even in the shell-free runtime.
                name = create('--platform', platform, '-p', '127.0.0.1::9090',
                              '-e', 'METRICSHELL_FINAL_WAIT_MODE=immediate', runtime,
                              '--ingestion-transport=http', '--http-ingestion-listen=127.0.0.1:9091',
                              '--exposition-listen=0.0.0.0:9090', '--', '/metricshell',
                              '--ingestion-transport=http', '--http-ingestion-listen=127.0.0.1:9093',
                              '--exposition-listen=127.0.0.1:9092', '--final-wait-mode=duration',
                              '--final-wait-duration=8s', '--', '/metricshell', '--version')
                runtime_port = run('port', name, '9090/tcp').stdout.strip().rsplit(':', 1)[1]
                for _ in range(20):
                    try:
                        fetch('http://127.0.0.1:' + runtime_port + '/metrics')
                        break
                    except (urllib.error.URLError, http.client.HTTPException, ConnectionResetError):
                        time.sleep(0.1)
                else:
                    raise RuntimeError('runtime /metrics unavailable during finite workload')
                if run('inspect', '-f', '{{.State.Running}}', name).stdout.strip() != 'true':
                    raise RuntimeError('runtime workload exited before scrape')
                wait(name, 0)
                # The published example supplies /bin/sh; no local fixture or derived image.
                for code in [0, 17]:
                    name = create('--platform', platform, '--entrypoint', '/usr/local/bin/metricshell',
                                  '-e', 'METRICSHELL_FINAL_WAIT_MODE=immediate', example,
                                  '--', '/bin/sh', '-c', 'exit ' + str(code))
                    wait(name, code)
                name = create('--platform', platform, '-p', '127.0.0.1::9090', example)
                port = run('port', name, '9090/tcp').stdout.strip().rsplit(':', 1)[1]
                for _ in range(30):
                    try:
                        body = fetch('http://127.0.0.1:' + port + '/metrics').decode()
                        if 'queue_depth 1' in body:
                            break
                    except (urllib.error.URLError, http.client.HTTPException, ConnectionResetError):
                        pass
                    time.sleep(0.5)
                else:
                    raise RuntimeError('published example metric queue_depth 1 missing')
                if run('inspect', '-f', '{{.State.Running}}', name).stdout.strip() != 'true':
                    raise RuntimeError('workload exited before metrics scrape')
                run('stop', '--time', '35', name, timeout=45)
                wait(name, 143)
                print('RUNTIME/EXAMPLE PASS:', platform, 'version revision exit=0/17 metrics shutdown', flush=True)
        finally:
            signal.alarm(0)
            # Independent cleanup deadline even when the main deadline expired.
            cleanup_errors = []
            for name in containers:
                result = subprocess.run(docker + ['rm', '-f', name], env=env, capture_output=True,
                                        text=True, timeout=20)
                if result.returncode and 'No such container' not in result.stderr:
                    cleanup_errors.append(name)
            if cleanup_errors:
                raise RuntimeError('cleanup failed: ' + ', '.join(cleanup_errors))
    signal.alarm(0)
    print('PUBLISHED RELEASE SMOKE: PASS (containers and isolated configuration removed)', flush=True)


if __name__ == '__main__':
    try:
        main()
    except (RuntimeError, OSError, subprocess.SubprocessError, ValueError) as error:
        raise SystemExit('PUBLISHED RELEASE SMOKE: FAIL: ' + str(error))
