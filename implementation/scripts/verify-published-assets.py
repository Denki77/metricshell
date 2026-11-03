#!/usr/bin/env python3
"""Verify downloaded v0.2.2 assets using Python stdlib and OpenSSL 3.

Usage: verify-published-assets.py ASSET_DIRECTORY RELEASE_API_JSON
Set TRUSTED_RELEASE_PUBLIC_KEY_HEX only after independent key confirmation.
Without it, integrity is checked but publisher trust is explicitly UNCONFIRMED.
"""
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile


def verify(directory, metadata):
    release = json.loads(metadata.read_text())
    if release['tag_name'] != 'v0.2.2' or release['draft'] or release['prerelease']:
        raise ValueError('unexpected release identity')
    assets = {asset['name']: asset for asset in release['assets']}
    for name, asset in assets.items():
        if Path(name).name != name:
            raise ValueError('unsafe asset name')
        actual = hashlib.sha256((directory / name).read_bytes()).hexdigest()
        if asset.get('digest') != 'sha256:' + actual:
            raise ValueError('GitHub asset digest mismatch: ' + name)
    manifest = {}
    for line in (directory / 'SHA256SUMS').read_text().splitlines():
        digest, name = line.split()
        if name in manifest or Path(name).name != name or name not in assets:
            raise ValueError('invalid manifest subject: ' + name)
        if hashlib.sha256((directory / name).read_bytes()).hexdigest() != digest:
            raise ValueError('signed checksum mismatch: ' + name)
        manifest[name] = digest
    key = bytes.fromhex((directory / 'RELEASE_PUBLIC_KEY').read_text().strip())
    trusted = os.environ.get('TRUSTED_RELEASE_PUBLIC_KEY_HEX')
    if trusted and bytes.fromhex(trusted) != key:
        raise ValueError('independently trusted public key mismatch')
    if len(key) != 32:
        raise ValueError('invalid public key length')
    with tempfile.TemporaryDirectory(prefix='metricshell-verify-') as temporary:
        public = Path(temporary) / 'public.der'
        signature = Path(temporary) / 'signature.bin'
        public.write_bytes(bytes.fromhex('302a300506032b6570032100') + key)
        signature.write_bytes(bytes.fromhex((directory / 'SHA256SUMS.sig').read_text().strip()))
        subprocess.run(['openssl', 'pkeyutl', '-verify', '-pubin', '-keyform', 'DER', '-inkey', str(public),
                        '-rawin', '-in', str(directory / 'SHA256SUMS'), '-sigfile', str(signature)],
                       check=True, timeout=30)
    provenance = json.loads((directory / 'PROVENANCE.json').read_text())
    if provenance['version'] != '0.2.2' or provenance['revision'] != '4b19f7e20b2aab366c5cb69edd9e831bc7c89d58':
        raise ValueError('provenance identity mismatch')
    for subject in provenance['subjects']:
        if subject['sha256'] != manifest[subject['path']]:
            raise ValueError('provenance subject mismatch')
    sbom = json.loads((directory / 'SBOM.json').read_text())
    for arch in ['amd64', 'arm64']:
        name = 'metricshell-linux-' + arch
        if not any(c.get('name') == name and c.get('sha256') == manifest[name] for c in sbom['components']):
            raise ValueError('SBOM binary missing: ' + name)
    # MODULES.jsonl is a stream of pretty-printed JSON objects, not necessarily one object per line.
    raw = (directory / 'MODULES.jsonl').read_text().strip()
    decoder = json.JSONDecoder()
    modules = []
    while raw:
        module, end = decoder.raw_decode(raw)
        modules.append(module)
        raw = raw[end:].lstrip()
    for module in modules:
        components = [c for c in sbom['components'] if c.get('type') == 'go-module' and c['name'] == module['Path']]
        if len(components) != 1:
            raise ValueError('SBOM module coverage mismatch')
        component = components[0]
        if module.get('Main'):
            if component.get('version') != '0.2.2' or component.get('sha256') != manifest['go.mod']:
                raise ValueError('SBOM main module mismatch')
        elif component.get('version') != module.get('Version') or component.get('sum') != module.get('Sum'):
            raise ValueError('SBOM dependency mismatch')
    vulnerabilities = json.loads((directory / 'VULNERABILITIES.json').read_text())
    if (vulnerabilities['input_sha256'] != manifest['GOVULNCHECK.json'] or
            vulnerabilities['decision'] != 'pass' or vulnerabilities.get('policy_blocking_findings')):
        raise ValueError('vulnerability evidence mismatch')
    print('GitHub asset digests: PASS (' + str(len(assets)) + ' assets)')
    print('Ed25519 signature, signed checksums, SBOM, provenance, vulnerability evidence: PASS')
    print('Independent key trust: ' + ('CONFIRMED BY CALLER' if trusted else 'UNCONFIRMED (release key only)'))
    print('Ed25519-covered: ' + ', '.join(sorted(manifest)))
    print('Outside signed manifest: ' + ', '.join(sorted(set(assets) - set(manifest))))


if __name__ == '__main__':
    if len(sys.argv) != 3:
        raise SystemExit(__doc__)
    try:
        verify(Path(sys.argv[1]), Path(sys.argv[2]))
    except (OSError, ValueError, KeyError, subprocess.SubprocessError) as error:
        raise SystemExit('RELEASE ASSET VERIFICATION: FAIL: ' + str(error))
