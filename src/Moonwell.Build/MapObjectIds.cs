namespace Moonwell.Build
{
    /// <summary>The object ids the source map already uses: those its object files define and those it places.</summary>
    public static class MapObjectIds
    {
        private static readonly (string Kind, string Extension)[] Files =
        {
            ("unit", "w3u"),
            ("item", "w3t"),
            ("destructable", "w3b"),
            ("doodad", "w3d"),
            ("ability", "w3a"),
            ("buff", "w3h"),
            ("upgrade", "w3q"),
        };

        /// <summary>The id of every object in the source map's object files, main and skin, of every kind.</summary>
        /// <param name="project">The map project; its map folder holds the object files.</param>
        /// <returns>The ids; empty when the map has no object file.</returns>
        /// <exception cref="BuildException">An object file cannot be read.</exception>
        public static IReadOnlySet<string> Defined(MapProject project)
        {
            var ids = new HashSet<string>(StringComparer.Ordinal);
            foreach (var (kind, extension) in Files)
            {
                foreach (var name in new[] { "war3map." + extension, "war3mapSkin." + extension })
                {
                    var table = ObjectTable.Read(Path.Combine(project.MapFolder, name), kind);
                    foreach (var entry in table.Originals.Concat(table.News))
                    {
                        ids.Add(entry.Id);
                    }
                }
            }

            return ids;
        }

        /// <summary>The id of every unit, item, destructable and doodad the source map places.</summary>
        /// <param name="map">The opened source map, with its units and doodads.</param>
        /// <returns>The ids; empty when the map places nothing.</returns>
        public static IReadOnlySet<string> Placed(War3Net.Build.Map map)
        {
            var ids = new HashSet<string>(StringComparer.Ordinal);
            if (map.Units != null)
            {
                foreach (var unit in map.Units.Units)
                {
                    ids.Add(ObjectTable.TextOf(unit.TypeId));
                }
            }

            if (map.Doodads != null)
            {
                foreach (var doodad in map.Doodads.Doodads)
                {
                    ids.Add(ObjectTable.TextOf(doodad.TypeId));
                }

                foreach (var special in map.Doodads.SpecialDoodads)
                {
                    ids.Add(ObjectTable.TextOf(special.TypeId));
                }
            }

            return ids;
        }
    }
}
