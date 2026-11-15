"""Helper functions for integration tests."""

from fastapi import status
from fastapi.testclient import TestClient
from sqlalchemy.orm import Session

from dnd import schemas
from dnd.core import uncached_settings
from dnd.database.base_class import DbBase


def purge_table[ModelType: DbBase](client: TestClient, db: Session, model: type[ModelType], endpoint: str) -> None:
    """Completely purge a table and then check that nothing exists at the endpoint."""
    db.query(model).delete()
    db.commit()
    response = client.get(f"{uncached_settings.API_V1_STR}/{endpoint}")
    assert response.status_code == status.HTTP_200_OK, f"expected clean deletions for /{endpoint}"
    response_json = schemas.responses.GenericListResponse(**response.json())
    assert len(response_json.data) == 0, f"expected no data remaining for /{endpoint}"


def bulk_load_file(client: TestClient, test_data_path: str, endpoint: str) -> None:
    """Bulk load data for a given endpoint and verify no errors."""
    file_name = test_data_path.rsplit("/", 1)[-1]
    files = {
        "upload_file": (
            file_name,
            open(f"{test_data_path}", "rb"),
            "application/json",
        )
    }
    response = client.post(
        f"{uncached_settings.API_V1_STR}/{endpoint}/bulk",
        files=files,
        headers={"accept": "application/json"},
    )
    assert response.status_code == status.HTTP_200_OK
    response_json = schemas.responses.BulkLoadResponse(**response.json())
    assert response_json.totals.errored == 0
