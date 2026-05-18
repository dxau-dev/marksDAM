Here is a robust JSON design for your image recognition API response. I have structured this to be scalable and easy to parse, with clear constraints defined for each field.

### 1. The JSON Structure

This example shows a fully populated response.

```json
{"data": {"description": $the_description, "ocr": [ $SLICE_OF_OCR_WORDS ], "tags":[ $SLICE_OF_TAGS ] }, "meta": { $ANY_META_DATA_IN_JSON_FORMAT }

```

---

### 2. Field Constraints & Data Types

I have organized the constraints into a table for quick reference.

| Field | Data Type | Nullable? | Constraints | Notes |
| --- | --- | --- | --- | --- |
| **`description`** | `String` | **No** | • Min length: 1 char<br>

<br>• Max length: 255 chars (recommended) | Must be a complete sentence. If the AI cannot generate a description, return a fallback string or an error, never `null`. |
| **`ocr`** | `Array<String>` | **No** | • Min items: 0<br>

<br>• Unique items: Yes (Set) | Returns an empty array `[]` if no text is found. Do not return `null`. |
| **`tags`** | `Array<String>` | **No** | • Min items: 0<br>

<br>• Unique items: Yes (Set)<br>

<br>• Format: lowercase (recommended) | Returns an empty array `[]` if no tags are identified. |

---

### 3. Edge Case Examples

It is crucial to handle "empty" states consistently. In this design, arrays are used for sets; they simply become empty arrays rather than `null` values to prevent client-side crashing.

**Scenario: No text (OCR) or tags detected**

```json
{
  "data": {
    "description": "A blurry image of a grey wall.",
    "ocr": [],
    "tags": []
  }
}

```

**Scenario: Minimal valid response**

```json
{
  "data": {
    "description": "Unknown object.",
    "ocr": [],
    "tags": []
  }
}

```

### 4. Implementation Tips

* **Set vs. List:** Since you specified `OCR` and `Tags` are "sets," ensure your backend logic removes duplicates before serializing the JSON (e.g., preventing `["dog", "dog"]`).
* **Wrapper Object:** I wrapped the core fields in a `"data"` object. This is a best practice allowing you to add metadata (like `processing_time`, `model_version`, or `request_id`) later without breaking the API structure.
