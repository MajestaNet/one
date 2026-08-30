# Module: `education`

Optional managed module: **Education programs and courses**.

See [industry packages](../architecture/cdm-industry-packages.md) for the curated object set.

## Dependency

- `core` (must be installed)

## Version

`1.0.0`

## Objects

AcademicPeriod, AreaOfStudy, Program, Course, CourseSection, PreviousEducation, Scholarship, Internship, TestScore

Flexible `records` storage; `ownership=managed`, `package_name=education`.

Does **not** redefine spine apiNames (`Account`, `Contact`, `Product`, `Case`, `Opportunity`, `Campaign`, `Asset`, …).

## Enable

```http
POST /metadata/v1/packages/education/enable
```

## Soft-disable

```http
POST /metadata/v1/packages/education/disable
```
