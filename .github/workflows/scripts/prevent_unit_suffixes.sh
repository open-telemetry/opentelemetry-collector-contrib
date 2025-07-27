#!/bin/bash

# Script to find OpenTelemetry metrics that violate the spec by including units in metric names
# when units are already specified in metadata

set -euo pipefail

violations_found=0
total_metrics=0
temp_file=$(mktemp)

echo "🔍 Scanning OpenTelemetry Collector components for metric name violations..."
echo "Checking if metric names include units when they're already specified in metadata"
echo ""

# Function to get the text representation of a UCUM unit
get_unit_text() {
    local unit="$1"
    
    case "$unit" in
        # Time units
        "s") echo "second" ;;
        "ms") echo "millisecond" ;;
        "us"|"μs") echo "microsecond" ;;
        "ns") echo "nanosecond" ;;
        "min") echo "minute" ;;
        "h") echo "hour" ;;
        "d") echo "day" ;;
        
        # Data/Size units
        "By") echo "byte" ;;
        "bit") echo "bit" ;;
        "KiBy") echo "kilobyte" ;;
        "MiBy") echo "megabyte" ;;
        "GiBy") echo "gigabyte" ;;
        "TiBy") echo "terabyte" ;;
        
        # Frequency units
        "Hz") echo "hertz" ;;
        "kHz") echo "kilohertz" ;;
        "MHz") echo "megahertz" ;;
        "GHz") echo "gigahertz" ;;
        
        # Length units
        "m") echo "meter" ;;
        "cm") echo "centimeter" ;;
        "mm") echo "millimeter" ;;
        "km") echo "kilometer" ;;
        "in") echo "inch" ;;
        "ft") echo "foot" ;;
        
        # Weight/Mass units
        "g") echo "gram" ;;
        "kg") echo "kilogram" ;;
        "lb") echo "pound" ;;
        "oz") echo "ounce" ;;
        
        # Electrical units
        "A") echo "ampere" ;;
        "V") echo "volt" ;;
        "W") echo "watt" ;;
        "Ohm") echo "ohm" ;;
        
        # Energy units
        "J") echo "joule" ;;
        "cal") echo "calorie" ;;
        "eV") echo "electronvolt" ;;
        
        # Pressure units
        "Pa") echo "pascal" ;;
        "bar") echo "bar" ;;
        "atm") echo "atmosphere" ;;
        
        # Temperature units
        "K") echo "kelvin" ;;
        "Cel") echo "celsius" ;;
        "[degF]") echo "fahrenheit" ;;
        
        # Percentage and rates
        "%") echo "percent" ;;
        "Bps") echo "bytes_per_second" ;;
        "bps") echo "bits_per_second" ;;
        
        # Count/dimensionless units
        "1") echo "" ;;  # dimensionless - no text should appear
        
        # Unknown unit
        *) echo "" ;;
    esac
}

# Function to check a single metric
check_metric() {
    local metric_name="$1"
    local unit="$2"
    local file="$3"
    
    # Remove quotes from unit if present
    unit=$(echo "$unit" | sed 's/^"//;s/"$//')
    
    # Skip units that are inside brackets (e.g., {sample}, {batch}, {process})
    if [[ "$unit" =~ ^\{.*\}$ ]]; then
        return 0
    fi
    
    # Get the text representation of the unit
    local text_unit
    text_unit=$(get_unit_text "$unit")
    
    # Skip if no text representation or empty
    if [[ -z "$text_unit" ]]; then
        return 0
    fi
    
    # Convert metric name to lowercase for case-insensitive matching
    local lower_metric_name
    lower_metric_name=$(echo "$metric_name" | tr '[:upper:]' '[:lower:]')
    
    # Convert text_unit to lowercase for comparison
    local lower_text_unit
    lower_text_unit=$(echo "$text_unit" | tr '[:upper:]' '[:lower:]')
    
    # Check if the unit text appears as a suffix at the end of the metric name
    # Also check plural forms as suffixes
    if [[ "$lower_metric_name" == *"$lower_text_unit" ]] || [[ "$lower_metric_name" == *"${lower_text_unit}s" ]]; then
        echo "❌ VIOLATION FOUND:"
        echo "   File: $file"
        echo "   Metric: $metric_name"
        echo "   Unit: $unit (text: $text_unit)"
        echo "   Issue: Metric name ends with unit text '$text_unit' but unit is already specified in metadata"
        echo ""
        return 1
    fi
    
    return 0
}

# Find all metadata.yaml files and extract metrics
find . -name "metadata.yaml" -type f | while read -r file; do
    if ! grep -q "telemetry:" "$file" 2>/dev/null; then
        continue
    fi
    
    echo "📄 Processing: $file"
    
    # Extract metrics from the telemetry section using awk
    awk '
    /^telemetry:/ { in_telemetry = 1; next }
    /^[a-zA-Z]/ && in_telemetry && !/^  / { in_telemetry = 0 }
    in_telemetry && /^  metrics:/ { in_metrics = 1; next }
    in_telemetry && in_metrics && /^  [a-zA-Z]/ && !/^    / { in_metrics = 0 }
    in_telemetry && in_metrics && /^    [a-zA-Z_][a-zA-Z0-9_.]*:/ {
        metric_name = $1
        gsub(/:$/, "", metric_name)
        current_metric = metric_name
        next
    }
    in_telemetry && in_metrics && current_metric && /^      unit:/ {
        unit = $2
        for(i=3; i<=NF; i++) unit = unit " " $i
        print current_metric "|" unit "|" FILENAME
        current_metric = ""
    }
    ' "$file" >> "$temp_file"
    
done

# Process the extracted metrics
while IFS='|' read -r metric_name unit file_path; do
    if [[ -n "$metric_name" ]]; then
        ((total_metrics++))
        if ! check_metric "$metric_name" "$unit" "$file_path"; then
            ((violations_found++))
        fi
    fi
done < "$temp_file"

# Clean up
rm -f "$temp_file"

echo "📊 SUMMARY:"
echo "   Total metrics checked: $total_metrics"
echo "   Violations found: $violations_found"

if [[ $violations_found -gt 0 ]]; then
    echo ""
    echo "🚨 Found $violations_found metric(s) that violate the OpenTelemetry spec!"
    echo "   These metrics include unit names in their names even though units are specified in metadata."
    echo "   Consider renaming these metrics to remove the unit suffixes."
    exit 1
else
    echo ""
    echo "✅ No violations found! All metrics follow the OpenTelemetry spec correctly."
    exit 0
fi 