# Почему MetricShell

[English version](../../docs/01-problem/why-metricshell.md)

MetricShell не заменяет Prometheus, Pushgateway, OpenTelemetry, sidecar exporter или textfile collector.

Проект исследует альтернативную модель для случаев, где существующие подходы требуют лишних изменений приложения,
дополнительной инфраструктуры либо специфичного для среды развёртывания.

Цель — проверить, может ли переиспользуемый runtime-слой дать более простую эксплуатационную модель, сохраняя полную
совместимость с pull-экосистемой Prometheus.
