# The GitHub Ref is going to be a Pull ref, e.g., `refs/pull/123/head`
PR_ID="$(echo \"${GITHUB_REF}\" | sed -e 's|refs/pull/\([0-9]*\)/.*|\1|')"

echo "id=$PR_ID" >> $GITHUB_OUTPUT
