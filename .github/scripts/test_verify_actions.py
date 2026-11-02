"""Regression tests for the action commit release gate (no network)."""
import importlib.util
import json
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('verify_actions', Path(__file__).with_name('verify-actions.py'))
verifier = importlib.util.module_from_spec(spec)
spec.loader.exec_module(verifier)
SHA = 'a' * 40


class ActionPinsTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.workflows = self.root / '.github/workflows'
        self.workflows.mkdir(parents=True)

    def workflow(self, text, name='ci.yml'):
        (self.workflows / name).write_text(text)

    def test_scans_all_workflows_and_preserves_subpath_actions(self):
        self.workflow(f'  - uses: "owner/repo/path@{SHA}" # pinned\n  - uses: ./local\n')
        self.workflow(f'    uses: owner/repo@{SHA}\n', 'release.yaml')
        self.assertEqual(verifier.action_refs(self.root), [('owner/repo', '', SHA), ('owner/repo', 'path', SHA)])

    def test_rejects_tags_short_shas_and_docker_refs(self):
        for ref in ('owner/repo@v1', 'owner/repo@abcdef', 'docker://alpine:latest'):
            with self.subTest(ref=ref):
                self.workflow(f'- uses: {ref}\n')
                with self.assertRaises(ValueError):
                    verifier.action_refs(self.root)

    def test_fails_closed_on_missing_commit_or_network_error(self):
        self.workflow(f'- uses: owner/repo@{SHA}\n')
        with patch.object(verifier.subprocess, 'run', side_effect=subprocess.CalledProcessError(22, 'curl')):
            with self.assertRaises(subprocess.CalledProcessError):
                verifier.verify(self.root)

    def test_requires_exact_sha_in_api_response(self):
        self.workflow(f'- uses: owner/repo@{SHA}\n')
        with patch.object(verifier.subprocess, 'run', return_value=subprocess.CompletedProcess('curl', 0, json.dumps({'sha': 'b' * 40}))):
            with self.assertRaises(ValueError):
                verifier.verify(self.root)

    def test_valid_action_entrypoints(self):
        self.workflow(f'- uses: owner/repo@{SHA}\n')
        for name in ('action.yml', 'action.yaml', 'Dockerfile'):
            with self.subTest(name=name), patch.object(verifier, 'github_api', side_effect=[
                    {'sha': SHA}, [{'path': name, 'type': 'file'}]]):
                verifier.verify(self.root)

    def test_missing_manifest_or_non_file_is_rejected(self):
        self.workflow(f'- uses: owner/repo@{SHA}\n')
        for contents in ([], [{'path': 'README.md', 'type': 'file'}],
                         [{'path': 'action.yml', 'type': 'dir'}],
                         [{'path': 'other/action.yml', 'type': 'file'}], {'message': 'error'}):
            with self.subTest(contents=contents), patch.object(verifier, 'github_api',
                    side_effect=[{'sha': SHA}, contents]):
                with self.assertRaises(ValueError):
                    verifier.verify(self.root)

    def test_action_in_subdirectory_uses_pinned_sha(self):
        self.workflow(f'- uses: owner/repo/nested/action@{SHA}\n')
        with patch.object(verifier, 'github_api', side_effect=[
                {'sha': SHA}, [{'path': 'nested/action/action.yaml', 'type': 'file'}]]) as api:
            verifier.verify(self.root)
        self.assertEqual(api.call_args_list[-1].args,
                         (f'owner/repo/contents/nested/action?ref={SHA}',))

    def test_shared_commit_does_not_skip_subdirectory_validation(self):
        self.workflow(f'- uses: owner/repo@{SHA}\n- uses: owner/repo/sub@{SHA}\n')
        with patch.object(verifier, 'github_api', side_effect=[
                {'sha': SHA}, [{'path': 'action.yml', 'type': 'file'}], []]) as api:
            with self.assertRaises(ValueError):
                verifier.verify(self.root)
        self.assertEqual(api.call_count, 3)
        self.assertEqual(api.call_args.args, (f'owner/repo/contents/sub?ref={SHA}',))

    def test_contents_api_errors_fail_closed(self):
        self.workflow(f'- uses: owner/repo@{SHA}\n')
        for status in (401, 403, 404, 429, 500):
            with self.subTest(status=status), patch.object(verifier.subprocess, 'run',
                    side_effect=[subprocess.CompletedProcess('curl', 0, json.dumps({'sha': SHA})),
                                 subprocess.CalledProcessError(22, 'curl', stderr=str(status))]) as run:
                with self.assertRaises(subprocess.CalledProcessError):
                    verifier.verify(self.root)
                self.assertTrue(run.call_args.kwargs['check'])

    def test_contents_network_and_invalid_json_fail_closed(self):
        self.workflow(f'- uses: owner/repo@{SHA}\n')
        for failure in (subprocess.CalledProcessError(6, 'curl'),
                        subprocess.CompletedProcess('curl', 0, 'not json')):
            with self.subTest(failure=failure), patch.object(verifier.subprocess, 'run',
                    side_effect=[subprocess.CompletedProcess('curl', 0, json.dumps({'sha': SHA})), failure]):
                with self.assertRaises((ValueError, subprocess.SubprocessError)):
                    verifier.verify(self.root)


if __name__ == '__main__':
    unittest.main()
