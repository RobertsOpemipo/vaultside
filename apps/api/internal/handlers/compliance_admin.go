package handlers

import (
    "net/http"

    "github.com/google/uuid"
    "github.com/jackc/pgx/v5/pgtype"

    "github.com/vaultside/vaultside/apps/api/internal/db"
    "github.com/vaultside/vaultside/apps/api/internal/httpx"
)

type heatmapCell struct {
    DepartmentID   string  `json:"department_id"`
    RegulationID   string  `json:"regulation_id"`
    Headcount      int64   `json:"headcount"`
    CompliantCount int64   `json:"compliant_count"`
    CoveragePct    float64 `json:"coverage_pct"`
}

type heatmapDepartment struct {
    ID   string `json:"id"`
    Name string `json:"name"`
}

type heatmapRegulation struct {
    ID   string `json:"id"`
    Key  string `json:"key"`
    Name string `json:"name"`
}

type heatmapResponse struct {
    Departments []heatmapDepartment `json:"departments"`
    Regulations []heatmapRegulation `json:"regulations"`
    Cells       []heatmapCell       `json:"cells"`
}

func toPgUUID(u uuid.UUID) pgtype.UUID {
    return pgtype.UUID{Bytes: u, Valid: true}
}

func ComplianceHeatmap(w http.ResponseWriter, r *http.Request) {
    q := httpx.MustQueries(r.Context())

    depts, err := q.ListDepartments(r.Context())
    if err != nil {
        httpx.Error(w, http.StatusInternalServerError, "departments_failed", "")
        return
    }
    regs, err := q.ListRegulations(r.Context())
    if err != nil {
        httpx.Error(w, http.StatusInternalServerError, "regulations_failed", "")
        return
    }

    outDepts := make([]heatmapDepartment, 0, len(depts))
    for _, d := range depts {
        outDepts = append(outDepts, heatmapDepartment{ID: d.ID.String(), Name: d.Name})
    }
    outRegs := make([]heatmapRegulation, 0, len(regs))
    for _, rg := range regs {
        outRegs = append(outRegs, heatmapRegulation{ID: rg.ID.String(), Key: rg.Key, Name: rg.Name})
    }

    cells := make([]heatmapCell, 0, len(depts)*len(regs))
    for _, d := range depts {
        deptUUID := toPgUUID(d.ID)
        headcount, err := q.CountDepartmentHeadcount(r.Context(), deptUUID)
        if err != nil {
            httpx.Error(w, http.StatusInternalServerError, "headcount_failed", "")
            return
        }
        for _, rg := range regs {
            n, err := q.CountCompliantInDepartmentForRegulation(r.Context(),
                db.CountCompliantInDepartmentForRegulationParams{
                    DepartmentID: deptUUID,
                    RegulationID: rg.ID,
                })
            if err != nil {
                httpx.Error(w, http.StatusInternalServerError, "compliance_failed", "")
                return
            }
            pct := 0.0
            if headcount > 0 {
                pct = float64(n) / float64(headcount) * 100.0
            }
            cells = append(cells, heatmapCell{
                DepartmentID:   d.ID.String(),
                RegulationID:   rg.ID.String(),
                Headcount:      headcount,
                CompliantCount: n,
                CoveragePct:    pct,
            })
        }
    }

    httpx.JSON(w, http.StatusOK, heatmapResponse{
        Departments: outDepts,
        Regulations: outRegs,
        Cells:       cells,
    })
}

type expiringItem struct {
    ID             string `json:"id"`
    UserID         string `json:"user_id"`
    UserFullName   string `json:"user_full_name"`
    UserEmail      string `json:"user_email"`
    CourseTitle    string `json:"course_title"`
    DepartmentName string `json:"department_name,omitempty"`
    VerifyCode     string `json:"verify_code"`
    ExpiresAt      string `json:"expires_at"`
}

func ExpiringCertificates(w http.ResponseWriter, r *http.Request) {
    q := httpx.MustQueries(r.Context())
    rows, err := q.ListExpiringCertificatesDetailed(r.Context())
    if err != nil {
        httpx.Error(w, http.StatusInternalServerError, "list_failed", "")
        return
    }
    out := make([]expiringItem, 0, len(rows))
    for _, row := range rows {
        item := expiringItem{
            ID:           row.ID.String(),
            UserID:       row.UserID.String(),
            UserFullName: row.UserFullName,
            UserEmail:    row.UserEmail,
            CourseTitle:  row.CourseTitle,
            VerifyCode:   row.VerifyCode,
        }
        if row.DepartmentName != nil {
            item.DepartmentName = *row.DepartmentName
        }
        if row.ExpiresAt.Valid {
            item.ExpiresAt = row.ExpiresAt.Time.Format("2006-01-02")
        }
        out = append(out, item)
    }
    httpx.JSON(w, http.StatusOK, map[string]any{
        "window_days": 60,
        "count":       len(out),
        "items":       out,
    })
}