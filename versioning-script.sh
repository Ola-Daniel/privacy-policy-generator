#!/bin/bash

# Get the latest tag
latest_tag=$(git describe --tags --abbrev=0)

# Split the version components
IFS='.' read -r major minor patch <<< "$latest_tag"

# Increment the patch version
patch=$((patch + 1))

# Output the new version
echo "$major.$minor.$patch"

