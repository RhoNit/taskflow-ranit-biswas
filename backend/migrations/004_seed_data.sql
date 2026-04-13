-- +goose Up
--
-- Seed user credentials:
--   Email:    test@example.com
--   Password: password123
--
-- The bcrypt hash below (cost 12) was generated with:
--   go run ./cmd/hashpw password123
--
-- UUIDs were generated with: uuidgen
--
INSERT INTO users (id, name, email, password)
VALUES (
    '77880cdf-01f5-426d-a8fc-2bff70c9b766',
    'Test User',
    'test@example.com',
    '$2a$12$iUovd.hNj4cHJc0Yqji2wubOvuCRRICi2uw/X65sqFxZImQ0Qvvw.'
) ON CONFLICT (email) DO NOTHING;

INSERT INTO projects (id, name, description, owner_id)
VALUES (
    '5ff382e1-a72c-47c4-9f5d-96e2ea307feb',
    'Demo Project',
    'A sample project to get started with TaskFlow',
    '77880cdf-01f5-426d-a8fc-2bff70c9b766'
) ON CONFLICT DO NOTHING;

INSERT INTO tasks (id, title, description, status, priority, project_id, creator_id, assignee_id, due_date)
VALUES
    ('f6acc292-4153-4904-9a22-87f74e014444', 'Set up CI/CD pipeline', 'Configure GitHub Actions for automated testing', 'todo', 'high', '5ff382e1-a72c-47c4-9f5d-96e2ea307feb', '77880cdf-01f5-426d-a8fc-2bff70c9b766', '77880cdf-01f5-426d-a8fc-2bff70c9b766', '2026-04-30'),
    ('76f2aa83-0c03-4115-ae41-81196adc1ad4', 'Write API documentation', 'Document all REST endpoints with examples', 'in_progress', 'medium', '5ff382e1-a72c-47c4-9f5d-96e2ea307feb', '77880cdf-01f5-426d-a8fc-2bff70c9b766', '77880cdf-01f5-426d-a8fc-2bff70c9b766', NULL),
    ('867f6ee5-c701-49ce-9a1f-52c952b5f69a', 'Design database schema', 'Create ER diagram and define table relationships', 'done', 'low', '5ff382e1-a72c-47c4-9f5d-96e2ea307feb', '77880cdf-01f5-426d-a8fc-2bff70c9b766', NULL, '2026-04-15')
ON CONFLICT DO NOTHING;

-- +goose Down
DELETE FROM tasks WHERE project_id = '5ff382e1-a72c-47c4-9f5d-96e2ea307feb';
DELETE FROM projects WHERE id = '5ff382e1-a72c-47c4-9f5d-96e2ea307feb';
DELETE FROM users WHERE id = '77880cdf-01f5-426d-a8fc-2bff70c9b766';
