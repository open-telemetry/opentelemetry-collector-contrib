# The GitHub Ref is going to be a Pull ref, e.g., `refs/pull/123/head`
TAG="pr-$(echo \"${GITHUB_REF}\" | awk -F/ '{ print $3 }')-${GITHUB_SHA:0:7}-lumigo"

echo "tag=$TAG" >> $GITHUB_OUTPUT
