# Production deployment

[English version](../../docs/07-delivery/production-deployment.md)

Это единая operational-точка входа для MetricShell 0.2.2. MetricShell работает как PID 1, управляет одним process tree,
принимает instrumentation локально и отдаёт `GET /metrics` Prometheus. Workload не знает monitoring destination:
discovery и scrape configuration принадлежат инфраструктуре.

## Установка

Нужны Linux amd64 или arm64, `curl`, Python 3 и OpenSSL 3 с поддержкой Ed25519. Опубликованный
[VERIFYING.md](https://github.com/Denki77/metricshell/releases/download/v0.2.2/VERIFYING.md)
описывает offline-проверки evidence. Исполнимая процедура ниже реализует проверку подписи и бинарника.
Для полного аудита скачайте все файлы релиза перед запуском `sha256sum -c SHA256SUMS`.

Сначала получите публичный ключ сопровождающего через независимо аутентифицированный канал
(например, ранее проверенное подписанное сообщение или прямой канал с уже подтверждённой личностью).
Сверьте все 32 байта Ed25519-ключа и экспортируйте `TRUSTED_RELEASE_PUBLIC_KEY_HEX`: 64 шестнадцатеричных символа.
Ключ из того же релиза — только кандидат; ни он, ни checksum из того же источника не устанавливают независимое доверие.
Без независимого подтверждения ключа остановитесь до установки.

```sh
set -eu
version=0.2.2
revision=4b19f7e20b2aab366c5cb69edd9e831bc7c89d58
case "$(uname -m)" in
  x86_64) arch=amd64 ;;
  aarch64|arm64) arch=arm64 ;;
  *) echo "Unsupported architecture" >&2; exit 1 ;;
esac
: "${TRUSTED_RELEASE_PUBLIC_KEY_HEX:?Set the independently confirmed Ed25519 public key}"
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
cd "$work"
base="https://github.com/Denki77/metricshell/releases/download/v$version"
asset="metricshell-linux-$arch"
for file in "$asset" SHA256SUMS SHA256SUMS.sig RELEASE_PUBLIC_KEY VERIFYING.md; do
  curl --fail --location --show-error --connect-timeout 10 --max-time 120 \
    "$base/$file" --output "$file"
done
python3 - <<'VERIFY_KEY'
import os
from pathlib import Path
trusted = bytes.fromhex(os.environ['TRUSTED_RELEASE_PUBLIC_KEY_HEX'])
published = bytes.fromhex(Path('RELEASE_PUBLIC_KEY').read_text().strip())
if len(trusted) != 32 or trusted != published:
    raise SystemExit('Release public key does not match independent trust anchor')
# RFC 8410 SubjectPublicKeyInfo for Ed25519, then the raw 32-byte key.
Path('release-key.der').write_bytes(bytes.fromhex('302a300506032b6570032100') + trusted)
signature = bytes.fromhex(Path('SHA256SUMS.sig').read_text().strip())
if len(signature) != 64:
    raise SystemExit('Invalid Ed25519 signature length')
Path('signature.bin').write_bytes(signature)
VERIFY_KEY
openssl pkeyutl -verify -pubin -keyform DER -inkey release-key.der \
  -rawin -in SHA256SUMS -sigfile signature.bin
# Verify exactly the selected asset, without requiring other release files.
python3 - "$asset" <<'VERIFY_HASH'
import hashlib
from pathlib import Path
import sys
asset = sys.argv[1]
lines = [line.split() for line in Path('SHA256SUMS').read_text().splitlines()]
selected = [fields[0] for fields in lines if len(fields) == 2 and fields[1] == asset]
if len(selected) != 1 or hashlib.sha256(Path(asset).read_bytes()).hexdigest() != selected[0]:
    raise SystemExit('Missing, duplicate or mismatched binary checksum')
VERIFY_HASH
chmod 0755 "$asset"
test "$(./"$asset" --version)" = "metricshell version=$version revision=$revision"
sudo install -m 0755 "$asset" /usr/local/bin/metricshell
metricshell --version
```

Теги `ghcr.io/denki77/metricshell:0.2.2`, `metricshell-artifact:0.2.2` и `metricshell-example:0.2.2`
служат для поиска образов `linux/amd64` и `linux/arm64`. В production используйте immutable digests:

```sh
docker pull ghcr.io/denki77/metricshell@sha256:20d5b0cec5195d677f49b65bf315637bf0fed6899a936de527392f85df57f8ec
docker run --rm ghcr.io/denki77/metricshell@sha256:20d5b0cec5195d677f49b65bf315637bf0fed6899a936de527392f85df57f8ec --version
```

Artifact image имеет тот же digest и содержит `/metricshell` для multi-stage копирования. Минимальный runtime image
не содержит shell; добавьте workload в собственный application image. Опубликованный пример:
`ghcr.io/denki77/metricshell-example@sha256:56fd77bb5534549a12b1e1a7948a3cd2d63a08de4ee79bb32bbbb8b2a13e0f6b`.
Не используйте `latest` в production.

Для Kubernetes используйте release asset `metricshell-kubernetes-v0.2.2.tar.gz`: в нём уже подставлен example digest.
Job/CronJob/Deployment в `implementation/examples/kubernetes/production` — шаблоны.
В v0.2.2 файлы `OCI_DIGESTS`, `METADATA` и Kubernetes bundle не входят в manifest с Ed25519-подписью;
перед использованием прочтите [аудит артефактов](../../docs/07-delivery/releases/v0.2.2-audit.md).
Задним числом это не исправляется.

Воспроизводимая анонимная приёмка из репозитория (Python 3, Docker, native/QEMU для обеих архитектур):

```sh
make -C implementation published-release-smoke
```

Тест использует изолированные credentials, опубликованные digests, обе архитектуры, ограниченные ожидания и cleanup.
MetricShell из локальных исходников не собирается. Приватный пакет означает ошибку: владелец должен изменить
visibility соответствующего GHCR package на Public; авторизация не является обходным решением.

## Runtime

MetricShell должен быть PID 1; workload указывается после `--`. Он пересылает signals process group, reap'ит descendants
и сохраняет workload exit status. Exposition использует port 9090. Snapshot/Managed ingestion остаётся private:
предоставьте writable directory для Unix socket (обычно `/run/metricshell`) или atomic snapshot, оставив rootfs
read-only. Kubernetes API и service-account token не нужны.

Prometheus scrapes `GET /metrics`; `/healthz` и `/readyz` — probes и не считаются final scrape. Используйте discovery,
annotations или PodMonitor, а scrape timeout держите ниже interval. Для finite workload задайте mode `scrapes`, конечный
timeout и pod lifetime, достаточный хотя бы для одного eligible scrape.

## Security

Используйте фиксированный non-root UID/GID, read-only rootfs, drop `ALL` capabilities,
`allowPrivilegeEscalation: false`, `no-new-privileges` и seccomp `RuntimeDefault`. Отключите service-account token.
Открывайте только 9090. Managed socket храните в mode-0700 directory, доступном только workload. HTTP ingestion нельзя
публиковать в сеть.

## Resources и lifecycle

Начните с requests/limits в manifests и настройте их по измерениям. Ограничьте memory, PIDs и `nofile`, snapshot/decoded
bytes, scrape concurrency, Managed connections, queue, descriptors, series, label bytes и histogram buckets.

`terminationGracePeriodSeconds` должен быть больше внутреннего shutdown grace. Примеры используют 32 и 30 секунд с
двухсекундным reserve. Workload shutdown, admission close/drain, freeze, final Core installation и final wait bounded.
При недоступном Prometheus timeout завершает ожидание, сохраняя исходный workload result.

## Локальная instrumentation

Clients по умолчанию используют `/run/metricshell/managed.sock`, поэтому shell workload не передаёт destination:

```sh
metricshell managed declare jobs counter "Processed jobs" worker
metricshell managed counter-initialize jobs 0 worker=batch
metricshell managed counter-add jobs 1 worker=batch
metricshell managed declare queue_depth gauge "Queued jobs"
metricshell managed gauge-set queue_depth 7
metricshell managed declare job_seconds histogram "Job duration" - 0.1 1 +Inf
metricshell managed histogram-observe job_seconds 0.42
```

PHP 5.4 example находится в `implementation/examples/clients/php54`. Оба client знают только local Unix socket.

## Troubleshooting

- Workload не стартует: проверьте `workload.start_failed`, path, permissions и exit 73.
- Malformed metrics/Core candidate rejected: смотрите bounded reason; last valid snapshot сохраняется.
- Resource rejection/overload: смотрите self-metrics и меняйте конкретный bound только после измерений.
- Final scrape timeout: проверьте discovery, readiness, interval и timeout; ожидание намеренно конечное.
- Bind failure: исключите collision адресов и проверьте writable/private socket directory.
- Managed `UNKNOWN`: операция могла commit'иться при потере ACK; выполняйте осознанную reconciliation.
- Prometheus не видит target: проверьте selectors/annotations/PodMonitor и network policy до 9090.
- Restart: новый Managed process начинает пустую epoch; MetricShell не хранит application state между запусками.

Correctness guarantees задаются specifications и tests. Resource limits — hard bounds; benchmark observations не SLO.
