#!/usr/bin/env bash
set -eo pipefail

find_release_pipelines_from_pr() {
  local REPO=$1
  local PR_NUM=$2
 
  if [ -z "$REPO" ] || [ -z "$PR_NUM" ]; then
    echo "please provide repo and PR number, for example: find_release_pipelines_from_pr konflux-ci/release-service-catalog 949" 
    return 1
  fi

  setup_workspace
  clone_and_checkout_pr "$REPO" "$PR_NUM" || return 1
  
  # Declare global variables
  declare -a FOUND_PIPELINENAMES
  declare -a FOUND_INTERNAL_PIPELINENAMES
  declare -a TEKTON_INTERNAL_TASKS
  declare -a TEKTON_MANAGED_TASKS
  declare -a TEKTON_MANAGED_PIPELINES
  declare -a TEKTON_INTERNAL_PIPELINES
  declare SELECT_ALL_TESTCASES=false
  
  # Find all tasks and pipelines and save them to arrays
  find_changed_tekton_tasks_pipelines

  if [ "$SELECT_ALL_TESTCASES" = true ]; then
    echo -n "release-pipelines"
    cleanup_workspace
    return 0
  fi
  
  # Find managed pipelines for managed tasks and save them to $FOUND_PIPELINENAMES 
  if [ ${#TEKTON_MANAGED_TASKS[@]} -gt 0 ]; then
    for task in "${TEKTON_MANAGED_TASKS[@]}"; do
      find_pipelines_using_task "$task" "managed"
    done
  fi
  
  if [ ${#TEKTON_INTERNAL_TASKS[@]} -gt 0 ]; then
    for task in "${TEKTON_INTERNAL_TASKS[@]}"; do
      find_pipelines_using_task "$task" "internal"
    done
  fi
  
  # Deal with internal pipelines directly searched
  # FOUND_INTERNAL_PIPELINENAMES has values gotten by internal tasks
  if [ ${#TEKTON_INTERNAL_PIPELINES[@]} -gt 0 ]; then
    while IFS= read -r file; do
      local pipeline_name
      pipeline_name=$(yq e '.metadata.name' "$file")
    
      found=false
      for f in "${FOUND_INTERNAL_PIPELINENAMES[@]}"; do
        if [[ "$f" == "$pipeline_name" ]]; then
          found=true
          break
        fi
      done

      if [[ "$found" == false ]]; then
        FOUND_INTERNAL_PIPELINENAMES+=("$pipeline_name")
      fi
    done <<< $TEKTON_INTERNAL_PIPELINES
  fi
  
  declare -a TEMP_MANAGED_PIPELINENAMES

  # Map internal pipelines to managed pipelines
  for pipeline_name in "${FOUND_INTERNAL_PIPELINENAMES[@]}"; do
    case "$pipeline_name" in
      "create-advisory"|"check-embargoed-cves"|"get-advisory-severity"|"filter-already-released-advisory-images")
        TEMP_MANAGED_PIPELINENAMES+=("rh-advisories")
        ;;
      "update-fbc-catalog"|"publish-index-image-pipeline")
        TEMP_MANAGED_PIPELINENAMES+=("fbc-release")
        ;;
      "process-file-updates")
        TEMP_MANAGED_PIPELINENAMES+=("rh-advisories" "push-to-addons-registry" "rh-push-to-external-registry" "rh-push-to-registry-redhat-io")
        ;;
      "push-artifacts-to-cdn")
        TEMP_MANAGED_PIPELINENAMES+=("push-disk-images-to-cdn")
        ;;
      "simple-signing-pipeline")
        TEMP_MANAGED_PIPELINENAMES+=("fbc-release" "rh-advisories" "rh-push-to-external-registry" "rh-push-to-registry-redhat-io")
        ;;
      "blob-signing-pipeline")
        TEMP_MANAGED_PIPELINENAMES+=("release-to-github")
        ;;
      "push-disk-images")
        TEMP_MANAGED_PIPELINENAMES+=("push-disk-images-to-cdn" "push-disk-images-to-marketplaces")
        ;;
      *)
        continue
        ;;
    esac
  done

  # Process managed pipelines that were directly searched
  # FOUND_PIPELINENAMES has values gotten from managed tasks
  if [ ${#TEKTON_MANAGED_PIPELINES[@]} -gt 0 ] ; then
    while IFS= read -r file; do
      local pipeline_name
      pipeline_name=$(yq e '.metadata.name' "$file")
      
      # Add pipeline if not already present
      if [[ ! " ${FOUND_PIPELINENAMES[*]} " =~ " ${pipeline_name} " ]]; then
        FOUND_PIPELINENAMES+=("$pipeline_name")
      fi
    done < <(printf '%s\n' "${TEKTON_MANAGED_PIPELINES[@]}")
  fi

  if [ ${#TEMP_MANAGED_PIPELINENAMES[@]} -gt 0 ]; then
    while IFS= read -r pipeline_name; do
      # Add pipeline if not already present
      if [[ ! " ${FOUND_PIPELINENAMES[*]} " =~ " ${pipeline_name} " ]]; then
        FOUND_PIPELINENAMES+=("$pipeline_name")
      fi
    done < <(printf '%s\n' "${TEMP_MANAGED_PIPELINENAMES[@]}")
  fi

  if [ ${#FOUND_PIPELINENAMES[@]} -gt 0 ]; then
    export FOUND_PIPELINES="${FOUND_PIPELINENAMES[*]}"
  fi
  
  ALL_TESTCASES=("rh-advisories" "fbc-release" "release-to-github" "push-to-external-registry" "rhtap-service-push" "rh-push-to-registry-redhat-io" "rh-push-to-external-registry")
  SELECTED_TESTCASES=()

  for pplname in "${FOUND_PIPELINENAMES[@]}"; do
    for tc in "${ALL_TESTCASES[@]}"; do
        if [[ "$pplname" == "$tc" ]]; then
            SELECTED_TESTCASES+=("$pplname")
        fi
    done
  done
  if (( ${#SELECTED_TESTCASES[@]} > 0 )); then
    echo -n "${SELECTED_TESTCASES[*]}"
  else
    echo -n "happy-path"
  fi
   
  cleanup_workspace
}

setup_workspace() {
  local WORK_DIR=".tmp_pr_check"
  rm -rf "$WORK_DIR"
  mkdir "$WORK_DIR"
  cd "$WORK_DIR" || exit 1
}

clone_and_checkout_pr() {
  local repo=$1
  local pr_num=$2

  git clone --quiet "https://github.com/$repo.git" release-service-catalog
  cd release-service-catalog || exit 1

  if ! git fetch origin pull/$pr_num/head:pr_$pr_num > /dev/null 2>&1 ; then
    echo "Failed to checkout PR"
    cd ../.. || exit 1
    rm -rf ".tmp_pr_check"
    return 1
  fi
  if ! git checkout "pr_$pr_num" > /dev/null 2>&1; then
    echo "Failed to checkout PR branch"
    cd ../.. || exit 1
    rm -rf ".tmp_pr_check"
    return 1
  fi
}

find_changed_tekton_tasks_pipelines() {
  local FILES
  FILES=$(git diff --name-only origin/development...HEAD)
  if [ -z "$FILES" ]; then
    cd ../.. || exit 1
    rm -rf ".tmp_pr_check"
    return 0
  fi
      
  while IFS= read -r file; do
    # match the files under stepactions
    if echo "$file" | grep -q "^stepactions/"; then
      SELECT_ALL_TESTCASES=true
      break
    elif [[ "$file" =~ ^schema/dataKeys.json$ ]] && [ -f "$file" ]; then
      SELECT_ALL_TESTCASES=true
      break
    elif [[ "$file" =~ ^tasks/internal/[^/]+/[^/]+\.ya?ml$ ]] && [ -f "$file" ]; then
      if grep -q -E 'kind: *Task' "$file"; then
        TEKTON_INTERNAL_TASKS+=("$file")
      fi
    elif [[ "$file" =~ ^tasks/managed/[^/]+/[^/]+\.ya?ml$ ]] && [ -f "$file" ]; then
      if grep -q -E 'kind: *Task' "$file"; then
        TEKTON_MANAGED_TASKS+=("$file")
      fi
    elif [[ "$file" =~ ^pipelines/managed/[^/]+/[^/]+\.ya?ml$ ]] && [ -f "$file" ]; then
      if grep -q -E 'kind: *Pipeline' "$file"; then
        TEKTON_MANAGED_PIPELINES+=("$file")
      fi
    elif [[ "$file" =~ ^pipelines/internal/[^/]+/[^/]+\.ya?ml$ ]] && [ -f "$file" ]; then
      if grep -q -E 'kind: *Pipeline' "$file"; then
        TEKTON_INTERNAL_PIPELINES+=("$file")
      fi
    fi
  done <<< "$FILES"
}

find_pipelines_using_task() {
  local task_file="$1"  # e.g., tasks/internal/task1/task1.yaml
  local pipeline_dir="$2"
  # Get Task name
  if [[ ! -f "$task_file" ]]; then
    echo "Error: Task file not found: $task_file" >&2
    return 1
  fi

  for dir in "pipelines/$pipeline_dir" ; do
    if [ -d "$dir" ]; then
      while IFS= read -r pipeline_file; do
        if [ -f "$pipeline_file" ] && grep -q "value: *$task_file" "$pipeline_file"; then
          local pipeline_name
          pipeline_name=$(yq e '.metadata.name' "$pipeline_file")

          if [[ -z "$pipeline_name" || "$pipeline_name" == "null" ]]; then
            echo "Error: Could not extract pipeline name from $pipeline_file" >&2
            return 1
          fi

          found=false
          if [ "$pipeline_dir" = "managed" ]; then
            for f in "${FOUND_PIPELINENAMES[@]}"; do
              if [[ "$f" == "$pipeline_name" ]]; then
                found=true
                break
              fi
            done

            if [[ "$found" == false ]]; then
              FOUND_PIPELINENAMES+=("$pipeline_name")
            fi
          else
            for f in "${FOUND_INTERNAL_PIPELINENAMES[@]}"; do
              if [[ "$f" == "$pipeline_name" ]]; then
                found=true
                break
              fi
            done

            if [[ "$found" == false ]]; then
              FOUND_INTERNAL_PIPELINENAMES+=("$pipeline_name")
            fi
          fi
        fi
      done < <(grep -rl "taskRef:" "$dir"/*/*.yaml 2>/dev/null)
    fi
  done   
}

cleanup_workspace() {
  cd ../.. || exit 1
  rm -rf ".tmp_pr_check"
}

find_release_pipelines_from_pr "$1" "$2"
