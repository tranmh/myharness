import pytest
from app import app, db


@pytest.fixture
def client():
    app.config["TESTING"] = True
    app.config["SQLALCHEMY_DATABASE_URI"] = "sqlite:///:memory:"
    with app.app_context():
        db.create_all()
        yield app.test_client()
        db.drop_all()


def test_create_and_list_todo(client):
    response = client.post("/todos", json={"title": "Buy milk"})
    assert response.status_code == 201
    data = response.get_json()
    assert data["title"] == "Buy milk"

    response = client.get("/todos")
    assert response.status_code == 200
    todos = response.get_json()
    assert len(todos) == 1
    assert todos[0]["title"] == "Buy milk"
