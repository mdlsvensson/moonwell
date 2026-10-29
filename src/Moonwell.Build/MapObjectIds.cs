using Moonwell.MapFiles;

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
            foreach (var kind in Kinds.All)
            {
                foreach (var name in new[] { Kinds.MainName(kind), Kinds.SkinName(kind) })
                {
                    var table = SourceObjectFile.Read(Path.Combine(project.MapFolder, name), kind);
                    foreach (var entry in table.Entries)
                    {
                        ids.Add(SourceObjectFile.IdOf(entry));
                    }
                }
            }

            return ids;
        }
    }
}
