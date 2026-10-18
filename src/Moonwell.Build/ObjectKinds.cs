namespace Moonwell.Build
{
    /// <summary>The seven kinds of object-data file, with the file extension of each: the one table the build reads the kinds from.</summary>
    internal static class ObjectKinds
    {
        /// <summary>The file kind and extension of each object-data file; a hero is in the unit file.</summary>
        public static readonly (string Kind, string Extension)[] All =
        {
            ("unit", "w3u"),
            ("item", "w3t"),
            ("destructable", "w3b"),
            ("doodad", "w3d"),
            ("ability", "w3a"),
            ("buff", "w3h"),
            ("upgrade", "w3q"),
        };

        /// <summary>The name of the kind's main file, such as <c>war3map.w3u</c>.</summary>
        public static string MainName(string extension)
        {
            return "war3map." + extension;
        }

        /// <summary>The name of the kind's skin file, such as <c>war3mapSkin.w3u</c>.</summary>
        public static string SkinName(string extension)
        {
            return "war3mapSkin." + extension;
        }
    }
}
