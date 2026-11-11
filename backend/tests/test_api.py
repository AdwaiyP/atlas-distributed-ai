"""FastAPI API contract tests, isolated from a running PostgreSQL instance."""
from datetime import datetime, timezone
from types import SimpleNamespace
from uuid import uuid4
from unittest.mock import MagicMock

import pytest
from fastapi.testclient import TestClient

from backend.app.main import app, get_db
from backend.app.models.task import Task


@pytest.fixture
def db():
    session = MagicMock()
    app.dependency_overrides[get_db] = lambda: session
    yield session
    app.dependency_overrides.clear()


@pytest.fixture
def client(db):
    # No context manager: avoids invoking the production lifespan/create_all.
    return TestClient(app)


def test_health(client):
    response = client.get('/health')
    assert response.status_code == 200
    assert response.json() == {'status': 'healthy', 'service': 'atlas-api'}


def test_ready_success(client, db):
    response = client.get('/ready')
    assert response.status_code == 200
    assert response.json() == {'status': 'ready'}
    db.execute.assert_called_once()


def test_ready_database_failure(client, db):
    db.execute.side_effect = RuntimeError('database unavailable')
    response = client.get('/ready')
    assert response.status_code == 503
    assert response.json()['detail'] == 'Database unavailable'


@pytest.mark.parametrize('prompt', ['', 'x' * 4001])
def test_reject_invalid_prompt(client, db, prompt):
    response = client.post('/api/v1/tasks', json={'prompt': prompt})
    assert response.status_code == 422
    db.add.assert_not_called()
    db.commit.assert_not_called()


def test_reject_missing_prompt(client, db):
    response = client.post('/api/v1/tasks', json={})
    assert response.status_code == 422
    db.commit.assert_not_called()


def test_create_task(client, db):
    task_id = uuid4()
    created = datetime.now(timezone.utc)

    def simulate_refresh(task):
        assert isinstance(task, Task)
        task.id = task_id
        task.created_at = created
        task.status = 'queued'

    db.refresh.side_effect = simulate_refresh
    response = client.post('/api/v1/tasks', json={'prompt': 'Explain distributed queues'})
    assert response.status_code == 202
    data = response.json()
    assert data['task_id'] == str(task_id)
    assert data['status'] == 'queued'
    assert datetime.fromisoformat(data['created_at']) == created
    db.add.assert_called_once()
    assert db.add.call_args.args[0].prompt == 'Explain distributed queues'
    db.commit.assert_called_once()
    db.refresh.assert_called_once()


def test_get_existing_task(client, db):
    task_id = uuid4()
    created = datetime.now(timezone.utc)
    db.get.return_value = SimpleNamespace(
        id=task_id, status='completed', result='A generated response', created_at=created
    )
    response = client.get(f'/api/v1/tasks/{task_id}')
    assert response.status_code == 200
    assert response.json()['task_id'] == str(task_id)
    assert response.json()['status'] == 'completed'
    assert response.json()['result'] == 'A generated response'
    db.get.assert_called_once_with(Task, task_id)


def test_get_missing_task(client, db):
    task_id = uuid4()
    db.get.return_value = None
    response = client.get(f'/api/v1/tasks/{task_id}')
    assert response.status_code == 404
    assert response.json()['detail'] == 'Task not found'


def test_get_invalid_task_id(client, db):
    response = client.get('/api/v1/tasks/not-a-uuid')
    assert response.status_code == 422
    db.get.assert_not_called()
