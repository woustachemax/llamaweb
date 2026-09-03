.PHONY: setup backend ml frontend dev clean

setup:
	cd frontend && npm install
	python3 -m venv ml/.venv && ml/.venv/bin/pip install -r ml/requirements.txt

backend:
	cd backend && go run .

ml:
	cd ml && ../ml/.venv/bin/uvicorn app:app --host 0.0.0.0 --port 8000 --reload

frontend:
	cd frontend && npm run dev

dev:
	./run.sh

clean:
	rm -rf backend/data ml/checkpoints frontend/dist
