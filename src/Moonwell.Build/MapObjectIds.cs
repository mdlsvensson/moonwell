namespace Moonwell.Build
{
    /// <summary>The object ids the source map's object files already define.</summary>
    public static class MapObjectIds
    {
        /// <summary>The id of every object in the source map's object files, main and skin, of every kind.</summary>
        /// <param name="project">The map project; its map folder holds the object files.</param>
        /// <returns>The ids; empty when the map has no object file.</returns>
        /// <exception cref="BuildException">An object file cannot be read.</exception>
        public static IReadOnlySet<string> Defined(MapProject project)
        {
            var ids = new HashSet<string>(StringComparer.Ordinal);
            foreach (var (kind, extension) in ObjectKinds.All)
            {
                foreach (var name in new[] { ObjectKinds.MainName(extension), ObjectKinds.SkinName(extension) })
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
    }
}
