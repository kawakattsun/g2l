#/bin/bash

set -a
source $1
set +a

cdk deploy --profile $2
