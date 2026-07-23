<?php
// Minimal producer examples. Usage: php client.php TRANSPORT ENDPOINT VALUE
$transport = $argv[1] ?? 'http';
$endpoint = $argv[2] ?? '127.0.0.1:19090';
$value = $argv[3] ?? "metric 1\n";
switch ($transport) {
case 'file':
    $tmp = $endpoint . '.tmp.' . getmypid();
    file_put_contents($tmp, $value, LOCK_EX);
    rename($tmp, $endpoint); // atomic replacement in the same filesystem
    break;
case 'unix-stream':
case 'unix-dgram':
    $kind = $transport === 'unix-stream' ? STREAM_SOCK_STREAM : STREAM_SOCK_DGRAM;
    $s = socket_create(AF_UNIX, $kind, 0);
    socket_connect($s, $endpoint);
    socket_send($s, $value, strlen($value), 0);
    socket_close($s);
    break;
case 'http':
    $c = curl_init("http://$endpoint/ingest");
    curl_setopt_array($c, [CURLOPT_POST => true, CURLOPT_POSTFIELDS => $value,
        CURLOPT_RETURNTRANSFER => true, CURLOPT_TIMEOUT_MS => 1000]);
    if (curl_exec($c) === false || curl_getinfo($c, CURLINFO_RESPONSE_CODE) !== 204) exit(2);
    curl_close($c);
    break;
default:
    fwrite(STDERR, "PHP stdlib has no native gRPC/shared-memory/mmap producer API\n");
    exit(64);
}

