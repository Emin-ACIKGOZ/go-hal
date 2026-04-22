#!/bin/bash
# check-benchmark-regression.sh - Detect performance regressions in benchmarks
# Stores baseline benchmarks and compares new results against them
# Allows up to 10% variance (5% typical variance + 5% threshold)

set -e

BENCHMARK_FILE="benchmark_results.txt"
BASELINE_DIR=".benchmarks"
BASELINE_FILE="$BASELINE_DIR/baseline.txt"
THRESHOLD=10  # 10% degradation threshold

# Create baseline directory if it doesn't exist
mkdir -p "$BASELINE_DIR"

if [ ! -f "$BENCHMARK_FILE" ]; then
    echo "❌ Benchmark results file not found: $BENCHMARK_FILE"
    exit 1
fi

# Function to extract benchmark metrics
extract_metrics() {
    local file=$1
    # Extract all lines starting with "Benchmark" and capture name, ops, and ns/op
    grep "^Benchmark" "$file" | awk '{print $1, $3, $5}' | sed 's/ns\/op//' | sed 's/ops\/sec//' | sed 's/B\/op//' | sed 's/allocs\/op//'
}

# If no baseline exists, create one
if [ ! -f "$BASELINE_FILE" ]; then
    echo "📊 Creating baseline benchmark file..."
    extract_metrics "$BENCHMARK_FILE" > "$BASELINE_FILE"
    echo "✓ Baseline created: $BASELINE_FILE"
    echo "  Next CI run will compare against this baseline."
    git add "$BASELINE_FILE"
    exit 0
fi

echo "📊 Comparing benchmarks against baseline..."
echo ""

current_metrics=$(extract_metrics "$BENCHMARK_FILE")
baseline_metrics=$(cat "$BASELINE_FILE")

regressions_found=0

# Compare each current benchmark with baseline
while IFS=' ' read -r bench_name bench_ops bench_ns; do
    baseline_line=$(echo "$baseline_metrics" | grep "^$bench_name " | head -1)

    if [ -z "$baseline_line" ]; then
        echo "⚠️  New benchmark detected: $bench_name"
        continue
    fi

    baseline_ns=$(echo "$baseline_line" | awk '{print $3}')

    # Calculate percentage change (allowing for floating point)
    # Formula: ((new - old) / old) * 100
    if (( $(echo "$baseline_ns > 0" | bc -l) )); then
        change_pct=$(echo "scale=2; (($bench_ns - $baseline_ns) / $baseline_ns) * 100" | bc -l)

        # Check if regression exceeds threshold
        if (( $(echo "$change_pct > $THRESHOLD" | bc -l) )); then
            echo "❌ REGRESSION: $bench_name"
            echo "   Baseline: ${baseline_ns}ns"
            echo "   Current:  ${bench_ns}ns"
            echo "   Change:   ${change_pct}% (threshold: ${THRESHOLD}%)"
            regressions_found=1
        elif (( $(echo "$change_pct > 5" | bc -l) )); then
            echo "⚠️  Minor change: $bench_name (${change_pct}%)"
        else
            echo "✓ $bench_name: OK (${change_pct}%)"
        fi
    fi
done <<< "$current_metrics"

echo ""

if [ $regressions_found -eq 1 ]; then
    echo "❌ Benchmark regressions detected! Fix performance issues or update baseline."
    exit 1
fi

# Update baseline for next run
extract_metrics "$BENCHMARK_FILE" > "$BASELINE_FILE"
echo "✓ Baseline updated for next comparison"

exit 0
