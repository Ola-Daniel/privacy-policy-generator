#!/bin/bash

# Get the latest tag if available
latest_tag=$(git describe --tags --abbrev=0 2>/dev/null)

# Check if latest_tag is empty
if [ -z "$latest_tag" ]; then
  # No tags found, set initial version
  initial_version="1.0.0"
  echo "Initial version: $initial_version"
  exit 0
fi

# Split the version components
IFS='.' read -r major minor patch <<< "$latest_tag"

# Increment the patch version
patch=$((patch + 1))

# Output the new version
echo "$major.$minor.$patch"