package handlers

import (
    "net/http"
)

func SCIMListSchemas(w http.ResponseWriter, r *http.Request) {
    base := scimBaseURL(r)
    scimJSON(w, http.StatusOK, map[string]any{
        "schemas":      []string{"urn:ietf:params:scim:api:messages:2.0:ListResponse"},
        "totalResults": 2,
        "startIndex":   1,
        "itemsPerPage": 2,
        "Resources": []map[string]any{
            userSchema(base),
            groupSchema(base),
        },
    })
}

func SCIMGetSchema(w http.ResponseWriter, r *http.Request) {
    id := r.URL.Path[len("/scim/v2/Schemas/"):]
    base := scimBaseURL(r)
    switch id {
    case "urn:ietf:params:scim:schemas:core:2.0:User":
        scimJSON(w, http.StatusOK, userSchema(base))
    case "urn:ietf:params:scim:schemas:core:2.0:Group":
        scimJSON(w, http.StatusOK, groupSchema(base))
    default:
        scimError(w, http.StatusNotFound, "schema not found")
    }
}

func SCIMListResourceTypes(w http.ResponseWriter, r *http.Request) {
    base := scimBaseURL(r)
    scimJSON(w, http.StatusOK, map[string]any{
        "schemas":      []string{"urn:ietf:params:scim:api:messages:2.0:ListResponse"},
        "totalResults": 2,
        "startIndex":   1,
        "itemsPerPage": 2,
        "Resources": []map[string]any{
            {
                "schemas":     []string{"urn:ietf:params:scim:schemas:core:2.0:ResourceType"},
                "id":          "User",
                "name":        "User",
                "endpoint":    "/Users",
                "description": "User Account",
                "schema":      "urn:ietf:params:scim:schemas:core:2.0:User",
                "meta":        map[string]any{"location": base + "/ResourceTypes/User"},
            },
            {
                "schemas":     []string{"urn:ietf:params:scim:schemas:core:2.0:ResourceType"},
                "id":          "Group",
                "name":        "Group",
                "endpoint":    "/Groups",
                "description": "Group",
                "schema":      "urn:ietf:params:scim:schemas:core:2.0:Group",
                "meta":        map[string]any{"location": base + "/ResourceTypes/Group"},
            },
        },
    })
}

func userSchema(base string) map[string]any {
    return map[string]any{
        "schemas":     []string{"urn:ietf:params:scim:schemas:core:2.0:Schema"},
        "id":          "urn:ietf:params:scim:schemas:core:2.0:User",
        "name":        "User",
        "description": "User Account",
        "attributes": []map[string]any{
            {"name": "userName", "type": "string", "multiValued": false, "required": true, "caseExact": false, "mutability": "readWrite", "returned": "default", "uniqueness": "server"},
            {"name": "externalId", "type": "string", "multiValued": false, "required": false, "caseExact": false, "mutability": "readWrite", "returned": "default", "uniqueness": "none"},
            {"name": "name", "type": "complex", "multiValued": false, "required": false, "subAttributes": []map[string]any{
                {"name": "formatted", "type": "string", "multiValued": false, "required": false, "mutability": "readWrite", "returned": "default", "uniqueness": "none"},
                {"name": "givenName", "type": "string", "multiValued": false, "required": false, "mutability": "readWrite", "returned": "default", "uniqueness": "none"},
                {"name": "familyName", "type": "string", "multiValued": false, "required": false, "mutability": "readWrite", "returned": "default", "uniqueness": "none"},
            }},
            {"name": "emails", "type": "complex", "multiValued": true, "required": false, "subAttributes": []map[string]any{
                {"name": "value", "type": "string", "multiValued": false, "required": false, "mutability": "readWrite", "returned": "default", "uniqueness": "none"},
                {"name": "primary", "type": "boolean", "multiValued": false, "required": false, "mutability": "readWrite", "returned": "default", "uniqueness": "none"},
                {"name": "type", "type": "string", "multiValued": false, "required": false, "mutability": "readWrite", "returned": "default", "uniqueness": "none"},
            }},
            {"name": "active", "type": "boolean", "multiValued": false, "required": false, "mutability": "readWrite", "returned": "default", "uniqueness": "none"},
        },
        "meta": map[string]any{"resourceType": "Schema", "location": base + "/Schemas/urn:ietf:params:scim:schemas:core:2.0:User"},
    }
}

func groupSchema(base string) map[string]any {
    return map[string]any{
        "schemas":     []string{"urn:ietf:params:scim:schemas:core:2.0:Schema"},
        "id":          "urn:ietf:params:scim:schemas:core:2.0:Group",
        "name":        "Group",
        "description": "Group",
        "attributes": []map[string]any{
            {"name": "displayName", "type": "string", "multiValued": false, "required": true, "caseExact": false, "mutability": "readWrite", "returned": "default", "uniqueness": "none"},
            {"name": "externalId", "type": "string", "multiValued": false, "required": false, "mutability": "readWrite", "returned": "default", "uniqueness": "none"},
            {"name": "members", "type": "complex", "multiValued": true, "required": false, "subAttributes": []map[string]any{
                {"name": "value", "type": "string", "multiValued": false, "required": false, "mutability": "readWrite", "returned": "default", "uniqueness": "none"},
                {"name": "display", "type": "string", "multiValued": false, "required": false, "mutability": "readOnly", "returned": "default", "uniqueness": "none"},
                {"name": "type", "type": "string", "multiValued": false, "required": false, "mutability": "readOnly", "returned": "default", "uniqueness": "none"},
            }},
        },
        "meta": map[string]any{"resourceType": "Schema", "location": base + "/Schemas/urn:ietf:params:scim:schemas:core:2.0:Group"},
    }
}