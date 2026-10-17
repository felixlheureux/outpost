#!/usr/bin/env python3
"""Seed a planning branch through GitHub; never overwrite an existing queue.

Run deliberately with --repo OWNER/REPO after an initial commit exists. This
helper does not switch the coding checkout, publish issues, or reset branches.
Runtime queue mutations must use the file's current SHA; this helper only creates
the missing initial file and preserves anything already present.
"""

import argparse
import base64
import json
import re
import subprocess
import sys
from urllib.parse import quote


def gh(endpoint, *, method='GET', payload=None, missing_ok=False):
    argv = ['gh', 'api', '--method', method, endpoint]
    if payload is not None:
        argv += ['--input', '-']
    result = subprocess.run(
        argv, input=None if payload is None else json.dumps(payload),
        capture_output=True, text=True, check=False,
    )
    if result.returncode:
        if missing_ok and re.search(r'HTTP 404\b', result.stderr):
            return None
        raise RuntimeError(result.stderr.strip() or 'GitHub request failed')
    return json.loads(result.stdout) if result.stdout.strip() else {}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--repo', required=True, help='Exact OWNER/REPOSITORY')
    parser.add_argument('--branch', default='outpost/queue')
    parser.add_argument('--path', default='queue.json')
    args = parser.parse_args()
    if not re.fullmatch(r'[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+', args.repo):
        parser.error('--repo must be OWNER/REPOSITORY')
    if (not args.branch or args.branch.startswith('-') or '..' in args.branch
            or any(c.isspace() for c in args.branch)):
        parser.error('invalid planning branch name')
    if (not args.path or args.path.startswith('/')
            or any(p in ('', '.', '..') for p in args.path.split('/'))):
        parser.error('queue path must be a safe repository-relative path')

    repo_path = 'repos/' + args.repo
    repository = gh(repo_path)
    default_branch = repository['default_branch']
    if args.branch == default_branch:
        parser.error('planning branch must differ from the coding/default branch')
    ref_path = repo_path + '/git/ref/heads/' + quote(args.branch, safe='')
    ref = gh(ref_path, missing_ok=True)
    if ref is None:
        base = gh(repo_path + '/git/ref/heads/' + quote(default_branch, safe=''))
        try:
            gh(repo_path + '/git/refs', method='POST', payload={
                'ref': 'refs/heads/' + args.branch, 'sha': base['object']['sha'],
            })
        except RuntimeError:
            # Lost acknowledgement or a concurrent creator: observe, never force.
            if gh(ref_path, missing_ok=True) is None:
                raise
        print('Planning branch exists: ' + args.branch)
    else:
        print('Preserving existing planning branch: ' + args.branch)

    contents_path = repo_path + '/contents/' + quote(args.path, safe='/')
    read_path = contents_path + '?ref=' + quote(args.branch, safe='')
    existing = gh(read_path, missing_ok=True)
    if existing is not None:
        if not isinstance(existing, dict) or existing.get('type') != 'file':
            raise RuntimeError('Existing queue path is not a regular repository file')
        print('Preserving existing queue; no contents changed: ' + args.path)
        print('Observed blob SHA: ' + existing['sha'])
        return
    content = json.dumps({'version': 1, 'issues': []}, indent=2) + '\n'
    payload = {
        'message': 'Initialize Outpost planning queue', 'branch': args.branch,
        'content': base64.b64encode(content.encode()).decode(),
    }
    try:
        result = gh(contents_path, method='PUT', payload=payload)
        print('Created empty queue: ' + args.path)
        print('Observed blob SHA: ' + result['content']['sha'])
    except RuntimeError:
        # A concurrent write or lost acknowledgement may already have created it.
        # Preserve the observed file. Do not retry this PUT without a fresh SHA.
        observed = gh(read_path, missing_ok=True)
        if observed is None or not isinstance(observed, dict) or observed.get('type') != 'file':
            raise
        print('Queue now exists; preserving it after an uncertain create response.')
        print('Observed blob SHA: ' + observed['sha'])


if __name__ == '__main__':
    try:
        main()
    except (RuntimeError, OSError, ValueError, KeyError) as error:
        print('Setup stopped: ' + str(error), file=sys.stderr)
        sys.exit(1)
