CODEOWNERS=".github/CODEOWNERS"

check_code_owner_existence() {
  MODULES=$(find . -type f -name "go.mod" -exec dirname {} \; | sort | grep -E '^./' | cut -c 3-)
  MISSING_COMPONENTS=0
  for module in ${MODULES}
  do
    if ! grep -q "^$module" "$CODEOWNERS"; then
      ((MISSING_COMPONENTS=MISSING_COMPONENTS+1))
      echo "\"$module\" not included in CODEOWNERS"
    fi
  done
  echo "there are $MISSING_COMPONENTS components not included in CODEOWNERS"
  if [ "$MISSING_COMPONENTS" -gt 0 ]; then
    exit 1
  fi
}

check_component_existence() {
  NOT_EXIST_COMPONENTS=0
  while IFS= read -r line
  do
    if [[ $line =~ ^[^#\*] ]]; then
      COMPONENT_PATH=$(echo "$line" | cut -d" " -f1)
      if [ ! -d "$COMPONENT_PATH" ]; then
        echo "\"$COMPONENT_PATH\" does not exist as specified in CODEOWNERS"
        ((NOT_EXIST_COMPONENTS=NOT_EXIST_COMPONENTS+1))
      fi
    fi
  done <"$CODEOWNERS"
  echo "there are $NOT_EXIST_COMPONENTS component(s) that do not exist as specified in CODEOWNERS"
  if [ "$NOT_EXIST_COMPONENTS" -gt 0 ]; then
    exit 1
  fi
}

if [[ "$1" == "check_code_owner_existence" ]];  then
  check_code_owner_existence
elif [[ "$1" == "check_component_existence" ]]; then
  check_component_existence
fi
