-- Seed data for testing the admin panel
-- This creates a realistic dataset with users, audit logs, and repositories
-- Run with: sqlite3 test.db < srepanel/database/testdata/seed_admin_panel.sql

-- Super admin user (Discord)
INSERT OR IGNORE INTO passwords (id, username, hash, salt, format, provider, is_super_admin, updated_at)
VALUES (
    100001,
    'admin_super',
    X'0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20', -- dummy hash
    X'a1a2a3a4a5a6a7a8a9aaabacadaeafb0', -- dummy salt
    1,
    'discord',
    1,
    CURRENT_TIMESTAMP
);

-- Regular users from different providers
INSERT OR IGNORE INTO passwords (id, username, hash, salt, format, provider, is_super_admin, updated_at)
VALUES
    -- Discord users
    (200001, 'alice_d1a2b3', X'0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20', X'a1a2a3a4a5a6a7a8a9aaabacadaeafb0', 1, 'discord', 0, datetime('now', '-30 days')),
    (200002, 'bob_c4d5e6', X'0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20', X'a1a2a3a4a5a6a7a8a9aaabacadaeafb0', 1, 'discord', 0, datetime('now', '-25 days')),
    (200003, 'charlie_f7g8h9', X'0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20', X'a1a2a3a4a5a6a7a8a9aaabacadaeafb0', 1, 'discord', 0, datetime('now', '-20 days')),

    -- GitHub users
    (300001, 'diana_i1j2k3', X'0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20', X'a1a2a3a4a5a6a7a8a9aaabacadaeafb0', 1, 'github', 0, datetime('now', '-15 days')),
    (300002, 'eve_l4m5n6', X'0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20', X'a1a2a3a4a5a6a7a8a9aaabacadaeafb0', 1, 'github', 0, datetime('now', '-10 days')),

    -- Google users
    (400001, 'frank_o7p8q9', X'0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20', X'a1a2a3a4a5a6a7a8a9aaabacadaeafb0', 1, 'google', 0, datetime('now', '-5 days')),

    -- GitLab users
    (500001, 'grace_r1s2t3', X'0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20', X'a1a2a3a4a5a6a7a8a9aaabacadaeafb0', 1, 'gitlab', 0, datetime('now', '-3 days'));

-- OAuth identities
INSERT OR IGNORE INTO oauth_identities (id, provider, username, first_login, last_login)
VALUES
    (100001, 'discord', 'admin_super', datetime('now', '-90 days'), datetime('now', '-1 hour')),
    (200001, 'discord', 'alice_d1a2b3', datetime('now', '-30 days'), datetime('now', '-2 days')),
    (200002, 'discord', 'bob_c4d5e6', datetime('now', '-25 days'), datetime('now', '-1 day')),
    (200003, 'discord', 'charlie_f7g8h9', datetime('now', '-20 days'), datetime('now', '-3 hours')),
    (300001, 'github', 'diana_i1j2k3', datetime('now', '-15 days'), datetime('now', '-5 days')),
    (300002, 'github', 'eve_l4m5n6', datetime('now', '-10 days'), datetime('now', '-12 hours')),
    (400001, 'google', 'frank_o7p8q9', datetime('now', '-5 days'), datetime('now', '-6 hours')),
    (500001, 'gitlab', 'grace_r1s2t3', datetime('now', '-3 days'), datetime('now', '-30 minutes'));

-- Repositories
INSERT OR IGNORE INTO repositories (name, admin_ids)
VALUES
    ('mario_kart_wii', '200001,200002'),
    ('super_mario_galaxy', '200001,300001'),
    ('zelda_twilight_princess', '300001,300002'),
    ('metroid_prime', '200003');

-- Audit log entries with various activities
INSERT INTO audit_logs (timestamp, user_id, username, action, resource_type, resource_name, details, ip_address, user_agent, success)
VALUES
    -- Recent successful logins
    (strftime('%s', datetime('now', '-30 minutes')) * 1000, 100001, 'admin_super', 'login', 'session', NULL, NULL, '192.168.1.100', 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/120.0.0.0', 1),
    (strftime('%s', datetime('now', '-1 hour')) * 1000, 200003, 'charlie_f7g8h9', 'login', 'session', NULL, NULL, '10.0.0.50', 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) Safari/17.0', 1),
    (strftime('%s', datetime('now', '-2 hours')) * 1000, 300002, 'eve_l4m5n6', 'login', 'session', NULL, NULL, '172.16.0.25', 'Mozilla/5.0 (X11; Linux x86_64) Firefox/121.0', 1),
    (strftime('%s', datetime('now', '-6 hours')) * 1000, 400001, 'frank_o7p8q9', 'login', 'session', NULL, NULL, '203.0.113.45', 'Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X)', 1),

    -- Failed login attempts
    (strftime('%s', datetime('now', '-45 minutes')) * 1000, NULL, 'unknown_user', 'login_failed', 'session', NULL, '{"reason": "invalid_credentials"}', '198.51.100.78', 'curl/8.0.1', 0),
    (strftime('%s', datetime('now', '-3 hours')) * 1000, NULL, 'attacker123', 'login_failed', 'session', NULL, '{"reason": "invalid_credentials"}', '185.220.101.45', 'python-requests/2.31.0', 0),
    (strftime('%s', datetime('now', '-12 hours')) * 1000, NULL, 'admin', 'login_failed', 'session', NULL, '{"reason": "invalid_credentials"}', '185.220.101.46', 'python-requests/2.31.0', 0),

    -- Account creations
    (strftime('%s', datetime('now', '-30 days')) * 1000, 200001, 'alice_d1a2b3', 'account_created', 'user', 'alice_d1a2b3', '{"provider": "discord"}', '192.168.1.50', 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/119.0.0.0', 1),
    (strftime('%s', datetime('now', '-25 days')) * 1000, 200002, 'bob_c4d5e6', 'account_created', 'user', 'bob_c4d5e6', '{"provider": "discord"}', '10.0.0.100', 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) Safari/17.0', 1),
    (strftime('%s', datetime('now', '-15 days')) * 1000, 300001, 'diana_i1j2k3', 'account_created', 'user', 'diana_i1j2k3', '{"provider": "github"}', '172.16.0.10', 'Mozilla/5.0 (X11; Linux x86_64) Firefox/120.0', 1),

    -- Password resets
    (strftime('%s', datetime('now', '-7 days')) * 1000, 200001, 'alice_d1a2b3', 'password_reset', 'user', 'alice_d1a2b3', NULL, '192.168.1.50', 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/120.0.0.0', 1),
    (strftime('%s', datetime('now', '-2 days')) * 1000, 300002, 'eve_l4m5n6', 'password_reset', 'user', 'eve_l4m5n6', NULL, '172.16.0.25', 'Mozilla/5.0 (X11; Linux x86_64) Firefox/121.0', 1),

    -- Access requests
    (strftime('%s', datetime('now', '-20 days')) * 1000, 200001, 'alice_d1a2b3', 'access_requested', 'repository', 'mario_kart_wii', NULL, '192.168.1.50', 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/119.0.0.0', 1),
    (strftime('%s', datetime('now', '-18 days')) * 1000, 200002, 'bob_c4d5e6', 'access_requested', 'repository', 'mario_kart_wii', NULL, '10.0.0.100', 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) Safari/17.0', 1),
    (strftime('%s', datetime('now', '-10 days')) * 1000, 300002, 'eve_l4m5n6', 'access_requested', 'repository', 'zelda_twilight_princess', NULL, '172.16.0.25', 'Mozilla/5.0 (X11; Linux x86_64) Firefox/120.0', 1),

    -- Access granted
    (strftime('%s', datetime('now', '-19 days')) * 1000, 100001, 'admin_super', 'access_granted', 'repository', 'mario_kart_wii', '{"target_user": "alice_d1a2b3"}', '192.168.1.100', 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/119.0.0.0', 1),
    (strftime('%s', datetime('now', '-17 days')) * 1000, 200001, 'alice_d1a2b3', 'access_granted', 'repository', 'mario_kart_wii', '{"target_user": "bob_c4d5e6"}', '192.168.1.50', 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/119.0.0.0', 1),
    (strftime('%s', datetime('now', '-9 days')) * 1000, 300001, 'diana_i1j2k3', 'access_granted', 'repository', 'zelda_twilight_princess', '{"target_user": "eve_l4m5n6"}', '172.16.0.10', 'Mozilla/5.0 (X11; Linux x86_64) Firefox/120.0', 1),

    -- Repository operations
    (strftime('%s', datetime('now', '-30 days')) * 1000, 200001, 'alice_d1a2b3', 'repo_created', 'repository', 'mario_kart_wii', NULL, '192.168.1.50', 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/119.0.0.0', 1),
    (strftime('%s', datetime('now', '-15 days')) * 1000, 300001, 'diana_i1j2k3', 'repo_created', 'repository', 'zelda_twilight_princess', NULL, '172.16.0.10', 'Mozilla/5.0 (X11; Linux x86_64) Firefox/120.0', 1),
    (strftime('%s', datetime('now', '-4 days')) * 1000, 200001, 'alice_d1a2b3', 'repo_updated', 'repository', 'mario_kart_wii', '{"field": "admin_ids"}', '192.168.1.50', 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/120.0.0.0', 1),

    -- Logouts
    (strftime('%s', datetime('now', '-4 hours')) * 1000, 200001, 'alice_d1a2b3', 'logout', 'session', NULL, NULL, '192.168.1.50', 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/120.0.0.0', 1),
    (strftime('%s', datetime('now', '-8 hours')) * 1000, 300001, 'diana_i1j2k3', 'logout', 'session', NULL, NULL, '172.16.0.10', 'Mozilla/5.0 (X11; Linux x86_64) Firefox/121.0', 1);

-- Add more recent activity for variety
INSERT INTO audit_logs (timestamp, user_id, username, action, resource_type, resource_name, details, ip_address, user_agent, success)
VALUES
    -- Last 24 hours activity
    (strftime('%s', datetime('now', '-23 hours')) * 1000, 200001, 'alice_d1a2b3', 'login', 'session', NULL, NULL, '192.168.1.50', 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/120.0.0.0', 1),
    (strftime('%s', datetime('now', '-22 hours')) * 1000, 200002, 'bob_c4d5e6', 'login', 'session', NULL, NULL, '10.0.0.100', 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) Safari/17.0', 1),
    (strftime('%s', datetime('now', '-18 hours')) * 1000, NULL, 'hacker', 'login_failed', 'session', NULL, '{"reason": "invalid_credentials"}', '45.142.212.61', 'python-requests/2.31.0', 0),
    (strftime('%s', datetime('now', '-16 hours')) * 1000, 200003, 'charlie_f7g8h9', 'login', 'session', NULL, NULL, '10.0.0.50', 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) Safari/17.0', 1),
    (strftime('%s', datetime('now', '-14 hours')) * 1000, 200003, 'charlie_f7g8h9', 'password_reset', 'user', 'charlie_f7g8h9', NULL, '10.0.0.50', 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) Safari/17.0', 1),
    (strftime('%s', datetime('now', '-10 hours')) * 1000, 300001, 'diana_i1j2k3', 'login', 'session', NULL, NULL, '172.16.0.10', 'Mozilla/5.0 (X11; Linux x86_64) Firefox/121.0', 1),
    (strftime('%s', datetime('now', '-8 hours')) * 1000, 300002, 'eve_l4m5n6', 'login', 'session', NULL, NULL, '172.16.0.25', 'Mozilla/5.0 (X11; Linux x86_64) Firefox/121.0', 1),
    (strftime('%s', datetime('now', '-5 hours')) * 1000, NULL, 'script_kiddie', 'login_failed', 'session', NULL, '{"reason": "invalid_credentials"}', '91.92.251.103', 'python-requests/2.31.0', 0);
