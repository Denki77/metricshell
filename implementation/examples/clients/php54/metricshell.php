<?php
// PHP 5.4-compatible stateless MetricShell Managed Aggregation client.
// Unknown means the request may have committed and MUST NOT be retried blindly.
function metricshell_managed_send($socket, $timeout, $request) {
    if (!is_string($socket) || $socket === '' || !is_numeric($timeout) || $timeout <= 0 || !is_array($request)) {
        return array('category' => 'local', 'reason' => 'invalid_client_configuration');
    }
    $payload = json_encode($request);
    if ($payload === false) return array('category' => 'local', 'reason' => 'encode');
    $connection = @stream_socket_client('unix://'.$socket, $errno, $errstr, $timeout);
    if (!$connection) return array('category' => 'transport', 'reason' => 'connect');
    stream_set_timeout($connection, (int)$timeout, (int)(($timeout - (int)$timeout) * 1000000));
    $frame = $payload."\n";
    $written = 0;
    while ($written < strlen($frame)) {
        $count = @fwrite($connection, substr($frame, $written));
        if ($count === false || $count === 0) {
            fclose($connection);
            return array('category' => $written === 0 ? 'transport' : 'unknown', 'reason' => 'write');
        }
        $written += $count;
    }
    $raw = @fgets($connection, 65538);
    $meta = stream_get_meta_data($connection);
    fclose($connection);
    if ($raw === false || !empty($meta['timed_out'])) return array('category' => 'unknown', 'reason' => 'missing_response');
    $response = json_decode($raw, true);
    if (!is_array($response) || !isset($response['version']) || $response['version'] !== 1 || !isset($response['outcome'])) {
        return array('category' => 'protocol', 'reason' => 'invalid_response');
    }
    $outcome = $response['outcome'];
    if ($outcome === 'committed') $category = 'accepted';
    elseif ($outcome === 'rejected' || $outcome === 'closed' || $outcome === 'cancelled') $category = 'rejected';
    elseif ($outcome === 'unknown') $category = 'unknown';
    elseif ($outcome === 'overloaded') $category = 'overload';
    elseif ($outcome === 'protocol') $category = 'protocol';
    else return array('category' => 'protocol', 'reason' => 'invalid_response_outcome');
    $result = array('category' => $category);
    if (isset($response['reason'])) $result['reason'] = $response['reason'];
    if (isset($response['generation'])) $result['generation'] = $response['generation'];
    if (isset($response['commit'])) $result['commit'] = $response['commit'];
    return $result;
}

if (basename(__FILE__) === basename($_SERVER['SCRIPT_FILENAME'])) {
    if ($argc < 5) {
        fwrite(STDERR, "usage: php metricshell.php SOCKET TIMEOUT OPERATION NAME [VALUE|TYPE HELP LABELS [BUCKET ...]] [LABEL=VALUE ...]\n");
        exit(2);
    }
    $socket = $argv[1]; $timeout = (float)$argv[2]; $operation = str_replace('-', '_', $argv[3]); $name = $argv[4];
    if ($operation === 'declare') {
        if ($argc < 8) { fwrite(STDERR, "declare requires TYPE HELP LABELS\n"); exit(2); }
        $labels = $argv[7] === '-' ? array() : explode(',', $argv[7]);
        $buckets = array(); for ($i = 8; $i < $argc; $i++) $buckets[] = $argv[$i];
        $request = array('version'=>1, 'op'=>'declare', 'name'=>$name, 'type'=>$argv[5], 'help'=>$argv[6], 'label_names'=>$labels, 'buckets'=>$buckets);
    } else {
        $operations = array('counter_initialize', 'counter_add', 'gauge_set', 'histogram_observe');
        if (!in_array($operation, $operations, true)) { fwrite(STDERR, "unknown operation\n"); exit(2); }
        if ($argc < 6) { fwrite(STDERR, "operation requires VALUE\n"); exit(2); }
        $numeric = (float)$argv[5];
        if (!is_numeric($argv[5]) || is_infinite($numeric) || is_nan($numeric)) { fwrite(STDERR, "invalid value\n"); exit(2); }
        $labels = array();
        for ($i = 6; $i < $argc; $i++) {
            $parts = explode('=', $argv[$i], 2);
            if (count($parts) !== 2 || $parts[0] === '' || array_key_exists($parts[0], $labels)) { fwrite(STDERR, "invalid label\n"); exit(2); }
            $labels[$parts[0]] = $parts[1];
        }
        $request = array('version'=>1, 'op'=>$operation, 'name'=>$name, 'value'=>$argv[5], 'labels'=>(object)$labels);
    }
    $result = metricshell_managed_send($socket, $timeout, $request);
    echo json_encode($result)."\n";
    $codes = array('accepted'=>0, 'local'=>2, 'rejected'=>3, 'overload'=>4, 'protocol'=>5, 'transport'=>6, 'unknown'=>7);
    exit(isset($codes[$result['category']]) ? $codes[$result['category']] : 5);
}
