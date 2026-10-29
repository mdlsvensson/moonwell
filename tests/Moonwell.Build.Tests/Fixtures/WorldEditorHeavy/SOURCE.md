# World Editor save with edits on every tab

Sven saved this map with World Editor on 2026-10-26, starting from Moonwell's template map
(wc3-cs `moonwell/template/MoonwellMap/map`), as the folder map `Documents\Warcraft III\Maps\ObjectEditorTest.w3m`.
Only its fourteen object files are copied here, unchanged, from the editor's `tests/fixtures/we-heavy/` (the same
files are in `tests/Moonwell.MapFiles.Tests/Fixtures/we-heavy/`). This is actual editor output.

The build's test `A_maps_own_object_files_are_staged_as_they_are` uses them as a map's source files with nothing
declared: World Editor's own bytes (the end marker after each modification, which World Editor sets to the object's
id; bytes after the two tables; a text that is not valid UTF-8) must stay.
