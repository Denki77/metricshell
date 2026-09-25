<?php
// PHP 5.4-compatible stateless operation client: it retains no registry or snapshot.
function metricshell_send($transport, $endpoint, $op, $metric, $value, $labels) {
    $payload = json_encode(array('version'=>1, 'op'=>$op, 'metric'=>$metric, 'value'=>$value, 'labels'=>(object)$labels));
    if ($transport === 'unix') {
        $fp = @stream_socket_client('unix://'.$endpoint, $errno, $errstr, 2);
        if (!$fp) return array('category'=>'transport_error', 'detail'=>'connect: '.$errno.' '.$errstr);
        stream_set_timeout($fp, 2); if (fwrite($fp, $payload."\n") === false) { fclose($fp); return array('category'=>'transport_error', 'detail'=>'write'); }
        $raw = fgets($fp, 65537); $meta = stream_get_meta_data($fp); fclose($fp);
        if ($raw === false || !empty($meta['timed_out'])) return array('category'=>'protocol_error', 'detail'=>'missing_ack');
    } else {
        $url = parse_url($endpoint); $port = isset($url['port']) ? $url['port'] : 80;
        $fp = @stream_socket_client('tcp://'.$url['host'].':'.$port, $errno, $errstr, 2);
        if (!$fp) return array('category'=>'transport_error', 'detail'=>'connect: '.$errno.' '.$errstr);
        $path = (isset($url['path']) ? rtrim($url['path'],'/') : '').'/v1/operations';
        fwrite($fp, "POST ".$path." HTTP/1.1\r\nHost: ".$url['host']."\r\nContent-Type: application/json\r\nConnection: close\r\nContent-Length: ".strlen($payload)."\r\n\r\n".$payload);
        $raw = stream_get_contents($fp); fclose($fp); $parts = explode("\r\n\r\n", $raw, 2); $raw = isset($parts[1]) ? $parts[1] : '';
    }
    $reply = json_decode($raw, true);
    if (!is_array($reply) || !array_key_exists('ok', $reply)) return array('category'=>'protocol_error', 'detail'=>'invalid_ack');
    if (empty($reply['ok'])) return array('category'=>'rejected', 'reason'=>isset($reply['error']) ? $reply['error'] : 'unknown');
    return array('category'=>'accepted', 'epoch'=>isset($reply['epoch']) ? $reply['epoch'] : null);
}
if (basename(__FILE__) === basename($_SERVER['SCRIPT_FILENAME'])) {
    if ($argc < 6) { fwrite(STDERR, "usage: php metricshell.php unix|http endpoint inc|set|observe metric value [k=v]\n"); exit(2); }
    $labels=array(); if (isset($argv[6])) { foreach (explode(',', $argv[6]) as $pair) { $kv=explode('=', $pair, 2); $labels[$kv[0]]=$kv[1]; } }
    $result=metricshell_send($argv[1],$argv[2],$argv[3],$argv[4],(float)$argv[5],$labels); echo json_encode($result)."\n";
    if ($result['category']==='accepted') exit(0);
    if ($result['category']==='transport_error') exit(3);
    if ($result['category']==='rejected') exit(4);
    exit(5);
}
