get_collector_version() {
   collector_module="$1"
   main_mod_file="$2"

   if grep -q "$collector_module" "$main_mod_file"; then
      grep "$collector_module" "$main_mod_file" | (read mod version;
         echo $version)
   else
      echo "Error: failed to retrieve the \"$collector_module\" version from \"$main_mod_file\"."
      exit 1
   fi
}

check_collector_version_correct() {
   collector_module="$1"
   collector_mod_version="$2"
   incorrect_version=0
   mod_files=$(find . -type f -name "go.mod")

   for mod_file in $mod_files; do
      if grep -q "$collector_module" "$mod_file"; then
         mod_line=$(grep "$collector_module" "$mod_file")
         version=$(echo "$mod_line" | cut -d" " -f2)
         
         # To account for module is on its own require line
         # version field is shifted right by 1
         if [ "$version" == "$collector_module" ]; then
            version=$(echo "$mod_line" | cut -d" " -f3)
         fi

         if [ "$version" != "$collector_mod_version" ]; then
            incorrect_version=$((incorrect_version+1))
            echo "Incorrect version \"$version\" of \"$collector_module\" is included in \"$mod_file\". It should be version \"$collector_mod_version\"."
         fi
      fi
   done

   echo "There are $incorrect_version incorrect \"$collector_module\" version(s) in the module files."
   if [ "$incorrect_version" -gt 0 ]; then
      exit 1
   fi
}

COLLECTOR_MODULE="go.opentelemetry.io/collector "
COLLECTOR_MODEL_MODULE="go.opentelemetry.io/collector/model"
MAIN_MOD_FILE="./go.mod"
COLLECTOR_MOD_VERSION=$(get_collector_version $COLLECTOR_MODULE $MAIN_MOD_FILE)

# Check the collector module version in each of the module files
check_collector_version_correct "$COLLECTOR_MODULE" "$COLLECTOR_MOD_VERSION"

# Check the collector model module version in each of the module files
check_collector_version_correct "$COLLECTOR_MODEL_MODULE" "$COLLECTOR_MOD_VERSION"
