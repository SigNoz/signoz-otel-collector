// Reference: https://atlasgo.io/atlas-schema/projects
variable "url" {
  type        = string
  description = "The fully qualified URL of the target database in the format postgres://{user}:{password}@{host}:{port}/{database}."
}

env {
  name = atlas.env
  // The URL of the target database.
  url = var.url
  // The URL of the Dev Database. Do not change name of the database in the url!!
  dev = "docker+postgres://_/atlasdev:latest/dev?search_path=public"
  // The URL of or reference to for the desired schema of this environment. 
  src = "file://schemas"
  migration {
    // The URL to the migration directory.
    dir = "file://migrations"
    // An optional name to control the schema that the revisions table resides in.
    revisions_schema = "atlas"
  }

  format {
    migrate {
      // Set custom formatting for migrate diff.
      diff = format("{{ sql . ' ' }}")
    }
  }
}

lint {
  // Schema changes like CREATE INDEX or DROP INDEX can cause the database to lock the table against write operations. 
  // Add the atlas:txmode none directive to the header to prevent this file from running in a transaction.
  concurrent_index {
    check_create = true
    check_drop   = true
    check_txmode = true
    error        = true
  }

  // Ensure consistency and readability in naming.
  naming {
    error   = true
    match   = "^[a-z_]+$"
    message = "must be lowercase and optionally contain a _"
    index {
      match   = "^[a-z]+_idx$"
      message = "must be lowercase and end with _idx"
    }
    foreign_key {
      match   = "^[a-z]+_fk$"
      message = "must be lowercase and end with _fk"
    }
  }

  // Prevent backward-incompatible changes, also known as breaking changes.
  incompatible {
    error = true
  }

  // Data-dependent changes are changes to a database schema that may succeed or fail, depending on the data that is stored in the database.
  data_depend {
    error = true
  }
}
