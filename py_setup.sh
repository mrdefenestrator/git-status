#!/usr/bin/env bash
set -eu

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

(
  cd $DIR
  pip3 install virtualenv
  virtualenv venv
  /bin/bash -c "source venv/bin/activate; pip3 install -r requirements.txt"
)
