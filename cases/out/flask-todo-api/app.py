from flask import Flask, request, jsonify
from models import db, Todo

app = Flask(__name__)
app.config["SQLALCHEMY_DATABASE_URI"] = "sqlite:///todos.db"
app.config["SQLALCHEMY_TRACK_MODIFICATIONS"] = False

db.init_app(app)

with app.app_context():
    db.create_all()


@app.route("/todos", methods=["GET"])
def list_todos():
    todos = Todo.query.all()
    return jsonify([t.to_dict() for t in todos])


@app.route("/todos", methods=["POST"])
def create_todo():
    data = request.get_json()
    todo = Todo(title=data["title"])
    db.session.add(todo)
    db.session.commit()
    return jsonify(todo.to_dict()), 201


@app.route("/todos/<int:id>", methods=["DELETE"])
def delete_todo(id):
    todo = Todo.query.get_or_404(id)
    db.session.delete(todo)
    db.session.commit()
    return "", 204


if __name__ == "__main__":
    app.run(debug=True)
