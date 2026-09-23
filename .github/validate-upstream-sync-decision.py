#!/usr/bin/env python3
"""Validate the sync decision schema before accepting an automated review."""
import json
import sys
from pathlib import Path


def validate(value, schema, path="$"):
    kind = schema["type"]
    valid_type = {
        "object": isinstance(value, dict),
        "array": isinstance(value, list),
        "string": isinstance(value, str),
    }.get(kind, False)
    if not valid_type:
        raise ValueError(f"{path}: expected {kind}")
    if "enum" in schema and value not in schema["enum"]:
        raise ValueError(f"{path}: invalid value")
    if kind == "object":
        for key in schema.get("required", []):
            if key not in value:
                raise ValueError(f"{path}.{key}: required field missing")
        properties = schema.get("properties", {})
        for key, item in value.items():
            if key not in properties:
                if schema.get("additionalProperties") is False:
                    raise ValueError(f"{path}.{key}: unexpected field")
            else:
                validate(item, properties[key], f"{path}.{key}")
    elif kind == "array":
        for index, item in enumerate(value):
            validate(item, schema["items"], f"{path}[{index}]")


def main():
    try:
        schema = json.loads(Path(sys.argv[1]).read_text(encoding="utf-8"))
        decision = json.loads(Path(sys.argv[2]).read_text(encoding="utf-8"))
        validate(decision, schema)
    except (OSError, ValueError, IndexError) as error:
        print(f"Invalid review output: {error}", file=sys.stderr)
        return 2
    if decision["decision"] != "resolved":
        print("Review remains blocked; continue repairing the candidate.", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())

