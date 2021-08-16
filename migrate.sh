#!/usr/bin/env sh

echo "Start to migrate $1/$2"
REMOTE_REPOS="$(git remote show)"
if [[ $REMOTE_REPOS != *"contrib-upstream"* ]]; then
  git remote add -f contrib-upstream https://github.com/open-telemetry/opentelemetry-collector-contrib.git
fi
if [[ $REMOTE_REPOS != *"contrib-origin"* ]]; then
  git remote add contrib-origin https://github.com/open-o11y/opentelemetry-collector-contrib.git
fi
git checkout -b $2-migration

echo "Deleting irrelevant files"
for dir in * .[^.]*; do
  if [ $dir != ".git" ] && [ $dir != $1 ]; then
    git rm -rq $dir
  fi
done
cd ./$1
for dir in *; do
  if [ $dir != $2 ]; then
    git rm -rq $dir
  fi
done
cd ..
rm .gitmodules
git add .gitmodules
git commit -qm "Delete irrelevant files"

echo "Merging Collector contrib into Collector core"
GIT_MERGE_VERBOSITY=0 git merge contrib-upstream/main --allow-unrelated-histories -m "Merge contrib into core"

echo "Adding Makefile and go module"
cd $1/$2
echo "include ../../Makefile.Common" > Makefile
go mod init github.com/open-telemetry/opentelemetry-collector-contrib/$1/$2
go mod tidy
git add *
git commit --amend -m "Add Makefile and go.mod"