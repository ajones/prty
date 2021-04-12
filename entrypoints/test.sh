#!/bin/bash

if [[ -z ${TEST_PARALLELISM} ]]; then
    TEST_PARALLELISM=5
fi

# Otherwise run the tests as usual
ginkgo --nodes=${TEST_PARALLELISM} -r --keepGoing --randomizeAllSpecs --randomizeSuites --failOnPending --cover --trace --progress --succinct .

# then watch if asked to
if [[ $1 = "watch" ]] || [[ $1 = "-w" ]]; then
  ginkgo watch -v -r --randomizeAllSpecs --failOnPending --cover --trace --progress --succinct .
fi

exit $?