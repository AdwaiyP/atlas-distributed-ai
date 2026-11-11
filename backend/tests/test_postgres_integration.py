"""ATLAS PostgreSQL integration tests; run only with ATLAS_TEST_DATABASE_URL."""
import os
import uuid
from datetime import datetime, timezone

import pytest
from sqlalchemy import create_engine, inspect, text
from sqlalchemy.orm import Session

URL = os.environ.get("ATLAS_TEST_DATABASE_URL")
pytestmark = pytest.mark.skipif(not URL, reason="Set ATLAS_TEST_DATABASE_URL for integration tests")


@pytest.fixture(scope="module")
def db_engine():
    if not URL:
        pytest.skip("ATLAS_TEST_DATABASE_URL not set")
    if "atlas_test" not in URL.split("?")[0].rsplit("/", 1)[-1]:
        pytest.fail("Refusing to run integration tests outside atlas_test database")
    engine = create_engine(URL, pool_pre_ping=True)
    with engine.connect() as conn:
        assert conn.execute(text("SELECT current_database() ")).scalar_one() == "atlas_test"
    yield engine
    engine.dispose()


@pytest.fixture(scope="module", autouse=True)
def schema(db_engine):
    # Import only after test URL is set, so the app's database module points at the test DB.
    from backend.app.core.database import Base
    from backend.app.models.task import Task  # noqa: F401
    Base.metadata.create_all(bind=db_engine)
    yield
    Base.metadata.drop_all(bind=db_engine)


def test_postgres_connectivity(db_engine):
    with db_engine.connect() as conn:
        assert conn.execute(text("SELECT 1")).scalar_one() == 1


def test_task_persists_between_sessions(db_engine):
    from backend.app.models.task import Task
    task_id = uuid.uuid4()
    with Session(db_engine) as session:
        task = Task(id=task_id, prompt="ATLAS integration test", status="queued")
        session.add(task)
        session.commit()
    with Session(db_engine) as session:
        saved = session.get(Task, task_id)
        assert saved is not None
        assert saved.prompt == "ATLAS integration test"
        assert saved.status == "queued"
        assert saved.attempts == 0
        assert saved.max_attempts == 3
        assert saved.created_at is not None
        session.delete(saved)
        session.commit()


def test_task_state_transition(db_engine):
    from backend.app.models.task import Task
    task_id = uuid.uuid4()
    with Session(db_engine) as session:
        session.add(Task(id=task_id, prompt="Transition test", status="queued"))
        session.commit()
    with Session(db_engine) as session:
        task = session.get(Task, task_id)
        task.status = "completed"
        task.result = "AI response"
        session.commit()
    with Session(db_engine) as session:
        task = session.get(Task, task_id)
        assert task.status == "completed"
        assert task.result == "AI response"
        session.delete(task)
        session.commit()
