#!/usr/bin/env python3
"""Validate pinned action availability, not major-version compatibility or publication."""
import json
import os
from pathlib import Path
import re
import subprocess
import sys
from urllib.parse import quote


def action_refs(root):
    refs = set()
    for path in sorted((root / '.github/workflows').rglob('*')):
        if path.suffix not in ('.yml', '.yaml'):
            continue
        for number, line in enumerate(path.read_text().splitlines(), 1):
            match = re.match(r'''^\s*(?:-\s*)?uses:\s*["']?([^\s"'#]+)''', line)
            if not match:
                continue
            ref = match[1]
            if ref.startswith('./'):
                continue
            if not re.fullmatch(r'[\w.-]+/[\w.-]+(?:/[\w./-]+)?@[0-9a-f]{40}', ref):
                raise ValueError(f'{path}:{number}: external action must use a full commit SHA: {ref}')
            repo, sha = ref.split('@')
            parts = repo.split('/')
            if any(part in ('', '.', '..') for part in parts):
                raise ValueError(f'{path}:{number}: invalid action path: {ref}')
            refs.add(('/'.join(parts[:2]), '/'.join(parts[2:]), sha))
    if not refs:
        raise ValueError('No external actions found')
    return sorted(refs)


def github_api(path):
    command = ['curl', '--fail-with-body', '--silent', '--show-error',
               '--retry', '3', '--connect-timeout', '10', '--max-time', '60',
               '-H', 'Accept: application/vnd.github+json',
               '-H', 'X-GitHub-Api-Version: 2022-11-28']
    # Feed credentials via stdin rather than exposing them in process arguments.
    token = os.environ.get('GH_TOKEN') or os.environ.get('GITHUB_TOKEN')
    config = ''
    if token:
        config = f'header = "Authorization: Bearer {token}"\n'
    result = subprocess.run(command + ['--config', '-',
                            f'https://api.github.com/repos/{path}'],
                            input=config, text=True, capture_output=True, check=True)
    return json.loads(result.stdout)


def verify(root):
    refs = action_refs(root)
    checked_commits = set()
    for repo, directory, sha in refs:
        if (repo, sha) not in checked_commits:
            commit = github_api(f'{repo}/commits/{sha}')
            if not isinstance(commit, dict) or commit.get('sha') != sha:
                raise ValueError(f'Commit SHA mismatch: {repo}@{sha}')
            checked_commits.add((repo, sha))
        # List the exact directory at the pinned commit. This checks all three
        # supported entrypoint names with one request, without swallowing 404s.
        contents = github_api(f'{repo}/contents/{quote(directory, safe="/")}?ref={sha}')
        expected = {f'{directory}/{name}' if directory else name
                    for name in ('action.yml', 'action.yaml', 'Dockerfile')}
        if not isinstance(contents, list) or not any(
                isinstance(item, dict) and item.get('type') == 'file'
                and item.get('path') in expected for item in contents):
            raise ValueError(f'No action.yml, action.yaml or Dockerfile: '
                             f'{repo}/{directory}@{sha}')
        action = f'{repo}/{directory}' if directory else repo
        print(f'OK {action}@{sha} (commit and action entrypoint)', flush=True)
    print(f'Validated {len(refs)} unique external actions')


if __name__ == '__main__':
    try:
        verify(Path(__file__).resolve().parents[2])
    except (ValueError, subprocess.SubprocessError) as error:
        print(f'Action verification failed: {error}', file=sys.stderr)
        sys.exit(1)
