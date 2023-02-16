#!/usr/bin/env bash
#
# Perform a copy of applications and components to a target namespace.

## This script was cpoied from https://github.com/hacbs-release/release-utils

SHORT_OPTS=a:,h
LONG_OPTS=applications:,all,help
OPTS=$(getopt --alternative --name copy-application --options $SHORT_OPTS --longoptions $LONG_OPTS -- "$@")

CLEANUP_ANNOTATIONS=(
    "\"appstudio.openshift.io/component-initial-build\"": "\"true\""
)
CLEANUP_KEYS=(
    ".metadata.finalizers",
    ".metadata.namespace",
    ".metadata.ownerReferences",
    ".metadata.resourceVersion",
    ".metadata.uid",
    ".spec.gitOpsRepository",
    ".status"
)
CLEANUP_COMMAND="jq 'del(${CLEANUP_KEYS[@]})' | jq '.metadata.annotations += {${CLEANUP_ANNOTATIONS[@]}}'"
NAMESPACED_NAME_REGEX="^[a-z0-9]([-a-z0-9]*[a-z0-9])?\/[a-z0-9]([-a-z0-9]*[a-z0-9])?$"
QUERY_ARGS="--no-headers -o custom-columns=NAMESPACE:.metadata.namespace,NAME:.metadata.name"
REQUIREMENTS="kubectl jq oc"
#######################################
# Check if all required tools are installed
# Globals:
#   REQUIREMENTS
# Arguments:
#   None
# Outputs:
#   Writes errors messages to stderr
#######################################
check_requirements() {
    for tool in $REQUIREMENTS; do
        if ! [ -x "$(command -v $tool)" ]; then
            echo "Error: $tool is not installed" >&2
            exit 3
        fi
    done

    oc status &>/dev/null || (echo "Error: Not logged in into a cluster" >&2 && exit 4)
}

#######################################
# Copy a given application to a the target workspace.
# Globals:
#   CLEANUP_COMMAND
#   NAMESPACED_NAME_REGEX
#   WORKSPACE
# Arguments:
#   Namespaced name of the application to copy
#######################################
copy_application() {
    if [[ ! "$1" =~ $NAMESPACED_NAME_REGEX ]]; then
        echo "Ignoring application '$1' as it's not a valid namespaced name"
    fi

    namespace=$(echo "$1" | cut -d '/' -f 1)
    application=$(echo "$1" | cut -d '/' -f 2)

    echo "Copying application '$application' to namespace '$WORKSPACE'"
    eval "kubectl get application $application --namespace=$namespace -o json | $CLEANUP_COMMAND | kubectl apply -n $WORKSPACE -f -"

    sleep 5

    echo "Copying application '$application' components"
    copy_components "$namespace" "$application"
}

#######################################
# Copy a list of applications to a target workspace. If no applications are
# supplied on the command line all the applications on the current namespace
# will get copied. If the --all option is used, all applications in all
# namespaces will be copied to the target namespace.
# Globals:
#   ALL
#   APPLICATIONS
#   QUERY_ARGS
# Arguments:
#   None
#######################################
copy_applications() {
    if [ -z "$APPLICATIONS" ]; then
        if [ "$ALL" = true ]; then
            # Create array with namespaced names of all applications in all namespaces
            readarray -t applications < <(kubectl get applications -A $QUERY_ARGS | awk '{print $1"/"$2}')
        else
            # Create array with namespaced names of all applications in the current namespace
            readarray -t applications < <(kubectl get applications $QUERY_ARGS | awk '{print $1"/"$2}')
        fi
    else
        # Create array with the list of namespaced names supplied on the command line
        IFS=',' read -ra applications <<<"$APPLICATIONS"
    fi

    for application in "${applications[@]}"; do
        copy_application "$application"
    done
}

#######################################
# Copy all the components associated with a given application to the
# target workspace.
# Globals:
#   CLEANUP_COMMAND
#   WORKSPACE
# Arguments:
#   Namespace of the application owning the components
#   Name of the application owning the components
#######################################
copy_components() {
    # Read all components into an array
    readarray -t components < <(kubectl get component -n "$1" -o jsonpath="{range .items[?(@.spec.application==\"$2\")]}{.metadata.name}{\"\n\"}{end}")

    for component in "${components[@]}"; do
        echo "Copying component '$component' to namespace '$WORKSPACE'"
        eval "kubectl get component $component --namespace=$1 -o json | $CLEANUP_COMMAND | kubectl apply -n $WORKSPACE -f -"
    done
}

#######################################
# Parse command line arguments.
# Globals:
#   ALL
#   APPLICATIONS
# Arguments:
#   None
# Outputs:
#   Writes errors messages to stderr
#######################################
parse_arguments() {
    eval set -- "$OPTS"

    while :; do
        case "$1" in
        -a | --applications)
            APPLICATIONS="$2"
            shift 2
            ;;
        -h | --help)
            show_help
            exit
            ;;
        --all)
            ALL=true
            shift
            ;;
        --)
            shift
            break
            ;;
        *) echo "Error: Unexpected option: $1" % >2 ;;
        esac
    done

    if [ "$ALL" = true ] && [ -n "$APPLICATIONS" ]; then
        echo "Error: Options '--all' and '-a' are mutually exclusive" >&2
        exit 2
    fi

    if [ "$#" -lt 1 ]; then
        echo "Error: Target workpace has to be specified as a positional argument" >&2
        exit 2
    fi

    if [ "$#" -gt 1 ]; then
        echo "Error: Only one positional argument (target workspace) is accepted" >&2
        exit 2
    fi

    WORKSPACE=${@:OPTIND:1}
}

#######################################
# Show help message.
# Globals:
#   None
# Arguments:
#   None
# Outputs:
#   Writes help message to stdout
#######################################
show_help() {
    echo "Usage:"
    echo "  copy-application [OPTION]... WORKSPACE"
    echo -e "\nCopy all applications and components from one workspace to another.\n"
    echo "Options:"
    echo "  -a,--applications if specified, only the given applications will be targeted"
    echo "  --all             copy all applications and components from all namespaces"
    echo "                    to the target one"
    echo -e "\n  -h, --help        display this help"
}

parse_arguments
check_requirements
copy_applications
