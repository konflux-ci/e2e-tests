# parse trace.json by function namespace
# Step 1: Print Function Names

jq '
map(select(.params.BeforeOrAfter == "Before" or .params.BeforeOrAfter == "After")) |
group_by(.functionName)[] |
{
  functionName: .[0].functionName
}
' trace.json

# Step 2: List "Before" and "After" Records
jq '
map(select(.params.BeforeOrAfter == "Before" or .params.BeforeOrAfter == "After")) |
group_by(.functionName)[] |
{
  functionName: .[0].functionName,
  records: 
    map(
      select(.params.BeforeOrAfter == "Before" or .params.BeforeOrAfter == "After") |
      { 
        BeforeOrAfter: .params.BeforeOrAfter,
        timestamp: .timestamp
      }
    )
}
' trace.json


# create a csv file with specific fields
cat trace.json | jq -r '
  .[] 
  | [
      .functionName, 
      .timestamp, 
      .params.ApplicationName, 
      .params.BeforeOrAfter, 
      .params.componentName, 
      .params.username, 
      .params.usernamespace
    ] 
  | @csv' > trace.csv


# list-of-diffs-per-ApplicationName-and-functionName.json
jq '
  # Define a function to convert timestamp strings to Unix timestamps, including fractional seconds
  def to_unixtimestamp:
    # Separate the date and time from the fractional seconds
    capture("(?<date>\\d{4}-\\d{2}-\\d{2}T\\d{2}:\\d{2}:\\d{2})\\.(?<fraction>\\d+)Z$")
    | (.date | strptime("%Y-%m-%dT%H:%M:%S") | mktime) + ("0." + .fraction | tonumber);

  # Separate "Before" and "After" records
  reduce .[] as $item ({"before": [], "after": []};
    if $item.params.BeforeOrAfter == "Before"
    then .before += [$item]
    else .after += [$item]
    end
  )

  # For each "Before" record, find the closest "After" record
  | .before[] as $before
  | [(.after[]
      | select(
          $before.params.usernamespace == .params.usernamespace and
          $before.functionName == .functionName and
          $before.params.ApplicationName == .params.ApplicationName and
          $before.timestamp < .timestamp
        )
      | {before: $before, after: ., diff: ((.timestamp | to_unixtimestamp) - ($before.timestamp | to_unixtimestamp))}
    )]
  | map(select(.diff != null))
  | select(length > 0)
  | min_by(.diff)
' trace.json | jq -s 'unique_by(.before.timestamp, .before.functionName, .before.params.usernamespace, .before.params.ApplicationName)'


# list-of-diffs-per-ApplicationName-and-functionName-sort-desc-by-diff.json
jq '
  # Define a function to convert timestamp strings to Unix timestamps, including fractional seconds
  def to_unixtimestamp:
    # Separate the date and time from the fractional seconds
    capture("(?<date>\\d{4}-\\d{2}-\\d{2}T\\d{2}:\\d{2}:\\d{2})\\.(?<fraction>\\d+)Z$")
    | (.date | strptime("%Y-%m-%dT%H:%M:%S") | mktime) + ("0." + .fraction | tonumber);

  # Separate "Before" and "After" records
  reduce .[] as $item ({"before": [], "after": []};
    if $item.params.BeforeOrAfter == "Before"
    then .before += [$item]
    else .after += [$item]
    end
  )

  # For each "Before" record, find the closest "After" record
  | .before[] as $before
  | [(.after[]
      | select(
          $before.params.usernamespace == .params.usernamespace and
          $before.functionName == .functionName and
          $before.params.ApplicationName == .params.ApplicationName and
          $before.timestamp < .timestamp
        )
      | {before: $before, after: ., diff: ((.timestamp | to_unixtimestamp) - ($before.timestamp | to_unixtimestamp))}
    )]
  | map(select(.diff != null))
  | select(length > 0)
  | min_by(.diff)
' trace.json | jq -s 'unique_by(.before.timestamp, .before.functionName, .before.params.usernamespace, .before.params.ApplicationName)' | jq 'sort_by(.diff) | reverse'



# list-of-diffs-grouped-by-ApplicationName-and-functionName.json
jq -s '
  flatten
  | map({
    ApplicationName: .before.params.ApplicationName,
    functionName: .before.functionName,
    diff: .diff
  })
  | group_by(.ApplicationName + " - " + .functionName)
  | map({
      ApplicationName: .[0].ApplicationName,
      functionName: .[0].functionName,
      records: .
    })
' list-of-diffs-per-ApplicationName-and-functionName.json


# Now sumnmerize diff per ApplicationName and functionName
# summerize-diffs-grouped-by-ApplicationName-and-functionName.json
jq ' 
  group_by(.ApplicationName, .functionName)
  | map({
      ApplicationName: .[0].ApplicationName,
      functionName: .[0].functionName,
      total_diff: map(.records[] | .diff) | add
    })
' list-of-diffs-grouped-by-ApplicationName-and-functionName.json

# Now sumnmerize diff per ApplicationName and functionName and sort descending by total_diff
# summerize-diffs-grouped-by-ApplicationName-and-functionName-sort-desc-by-total_diff.json
jq '
  group_by(.ApplicationName, .functionName)
  | map({
      ApplicationName: .[0].ApplicationName,
      functionName: .[0].functionName,
      total_diff: map(.records[] | .diff) | add
    })
  | sort_by(.total_diff) | reverse
' list-of-diffs-grouped-by-ApplicationName-and-functionName.json





#
# Now print the output as a table having the fields: "ApplicationName", "functionName", "total_diff" sorted descending by "total_diff" as a csv file
# summerize-diffs-grouped-by-ApplicationName-and-functionName-sort-desc-by-total_diff.csv
jq -r '
  group_by(.ApplicationName, .functionName)
  | map({
      ApplicationName: .[0].ApplicationName,
      functionName: .[0].functionName,
      total_diff: map(.records[] | .diff) | add
    })
  | sort_by(.total_diff) | reverse
  | map([.ApplicationName, .functionName, .total_diff])
  | .[] | @csv
' list-of-diffs-grouped-by-ApplicationName-and-functionName.json

