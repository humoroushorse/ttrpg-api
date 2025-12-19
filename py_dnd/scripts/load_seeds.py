"""Python script for populating seed data in a fresh database."""

import getpass
import os
from pathlib import Path

import requests
from pydantic import BaseModel


class TempReport(BaseModel):
    """Database seed load report."""

    created: int
    errored: int
    warning: int


report = TempReport(created=0, errored=0, warning=0)


# TODO: pure function
def update_report(report: TempReport, response: requests.Response, filename="") -> None:
    """Mutates a report by incrementing fields."""
    try:
        response_json: dict = response.json()
        totals: dict = response_json.get("totals", {})
        created = totals.get("created", 0)
        errored = totals.get("errored", 0)
        warning = totals.get("warning", 0)
        report.created += created
        report.errored += errored
        report.warning += warning

        def format_int(number: int) -> str:
            return f"{number:04d}"

        summary = {
            "created": format_int(created),
            "errored": format_int(errored),
            "warning": format_int(warning),
        }
        print(f"\t{summary} -> {filename}")  # noqa: T201
    except Exception as e:
        print(f"\tError parsing response for {filename}: {e}")
        report.errored += 1


def authenticate(session: requests.Session) -> bool:
    """Authenticate with the API using session cookies."""
    try:
        # Test if auth endpoint is available
        test_response = session.get(f"{api_root}/auth/user", timeout=5)
        if test_response.status_code == 401:
            # Auth is enabled, proceed with login
            username = input("Username: ")
            password = getpass.getpass("Password: ")
            
            auth_data = {
                "username": username,
                "password": password,
            }
            
            response = session.post(
                url=f"{api_root}/auth/session/token",
                data=auth_data,
                headers={"Content-Type": "application/x-www-form-urlencoded"}
            )
            
            if response.status_code == 200:
                print("✓ Authentication successful")
                return True
            else:
                print(f"✗ Authentication failed: {response.status_code}")
                print(f"Response: {response.text}")
                exit(1)
        else:
            print("✓ Auth not required or disabled")
            return False
    except Exception as e:
        print(f"⚠️  Auth service unavailable, proceeding without authentication: {e}")
        return False


# Configuration
data_root = Path("test_data/json/seeds")
api_root = "http://localhost:8001/api/v1"

# Create session for cookie handling
session = requests.Session()

# Authenticate and get token
print("Checking authentication...")
is_authenticated = authenticate(session)

headers = {
    "accept": "application/json",
}

# Map file patterns to API endpoints
endpoint_mapping = {
    "source.json": "sources",
    # "class.json": "classes",
    # "race.json": "races",
    "spells/": "spells",  # directory
    # "spells_to_classes/": "spell-to-class",  # directory
}

# Load order (dependencies first)
load_order = ["source.json", "spells/"]  # "class.json", "race.json", "spells_to_classes/"]


def load_files_for_endpoint(file_pattern: str, endpoint_url: str) -> None:
    """Load files for an endpoint - handles both individual files and directories."""
    if not data_root.exists():
        print(f"Data directory {data_root} not found!")
        return
    
    files_to_load = []
    
    if file_pattern.endswith("/"):  # Directory
        dir_path = data_root / file_pattern.rstrip("/")
        if dir_path.exists() and dir_path.is_dir():
            files_to_load = list(dir_path.glob("*.json"))
    else:  # Individual file
        file_path = data_root / file_pattern
        if file_path.exists():
            files_to_load = [file_path]
    
    if not files_to_load:
        print(f"No files found for: {file_pattern}")
        return
        
    print(f"\n################################################################################")
    print(f"# Loading {file_pattern.upper().rstrip('/')} files")
    print(f"################################################################################")
    
    for file_path in sorted(files_to_load):
        print(f"Loading: {file_path.name}")
        
        files = {
            "file": (
                file_path.name,
                open(file_path, "rb"),
                "application/json",
            )
        }
        
        try:
            response = session.post(
                url=f"{api_root}/{endpoint_url}/bulk", 
                files=files, 
                headers=headers
            )
            print(f"\tStatus: {response.status_code}")
            if response.status_code != 200:
                print(f"\tResponse: {response.text[:200]}...")
            else:
                try:
                    response_json = response.json()
                    print(f"\tAPI Response: {response_json}")
                except:
                    print(f"\tNon-JSON Response: {response.text[:100]}...")
            update_report(report, response, file_path.name)
        except Exception as e:
            print(f"\tError loading {file_path.name}: {e}")
            report.errored += 1
        finally:
            files["file"][1].close()


# Load all seed data in dependency order
for pattern in load_order:
    if pattern in endpoint_mapping:
        endpoint = endpoint_mapping[pattern]
        load_files_for_endpoint(pattern, endpoint)

################################################################################
# FINAL REPORT
################################################################################
print(f"\n{'='*80}")
print("FINAL REPORT:")
print(f"  Created: {report.created:04d}")
print(f"  Errored: {report.errored:04d}")
print(f"  Warning: {report.warning:04d}")
print(f"{'='*80}")
