-- Copyright 2021 The Go Authors. All rights reserved.
-- Use of this source code is governed by a BSD-style
-- license that can be found in the LICENSE file.

BEGIN;

ALTER TABLE symbol_search_documents ALTER COLUMN created_at SET NOT NULL;
ALTER TABLE symbol_search_documents ALTER COLUMN updated_at SET NOT NULL;

END;
