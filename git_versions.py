#!/usr/bin/env python3
import os
import sys
import semantic_version
from distutils.version import StrictVersion
import subprocess


FILE = '~/.git-status'

USAGE = '''\
git_versions.py [flags] [spec]
    Prints the latest versions of the git repositories

    -h, --help    Show this help
        --nofetch Do not fetch git repo
    
    spec          She semantic version specification upon which to match 
'''


def print_versions(spec=None, fetch=True):
    if spec:
        # Remove any 'v' prefix from requested version
        spec = spec.lstrip('v')

    # Read the repo list
    with open(os.path.expanduser(FILE), 'rb') as fp:
        repos = fp.read().decode('utf-8').split('\n')

    fetch_command = '(cd %s; git fetch;)'
    tags_command = '(cd %s; git tag -l)'
    for repo in repos:
        if not repo:
            # Skip blank lines
            continue

        if repo[0] == '#':
            # Skip commented lines but print whitespace
            print()
            continue

        if fetch:
            # Fetch from remote on the git repo
            subprocess.check_call(
                fetch_command % repo,
                shell=True,
                stdout=subprocess.DEVNULL,
                stderr=subprocess.DEVNULL
            )

        output = subprocess.check_output(tags_command % repo, shell=True)
        tags = output.decode('utf-8').split('\n')

        versions = []
        for tag in tags:
            # Remove any 'v' prefix
            tag = tag.lstrip('v')

            if not tag:
                # Skip empty tags
                continue

            try:
                sem_tag = semantic_version.Version(tag, partial=True)
            except ValueError:
                # Skip tags that are not semantic versions
                continue
            else:
                if spec:
                    s = semantic_version.Spec(spec)
                    if not s.match(sem_tag):
                        # Skip non-matching tags
                        continue

            versions.append(tag)

        try:
            versions.sort(key=StrictVersion)
        except ValueError:
            print('Invalid version in repo %s: %s' % (repo, versions))
            exit(-1)

        version = versions[-1] if versions else None

        print('%s: %s' % (
            os.path.basename(repo), version if version else 'no match'))


def main():
    if '-h' in sys.argv or '--help' in sys.argv:
        print(USAGE)
        exit(-1)

    fetch = '--nofetch' not in sys.argv
    args = [_ for _ in sys.argv[1:] if _[0] != '-']
    spec = args[0] if args else None

    print_versions(spec, fetch)


if __name__ == '__main__':
    main()
