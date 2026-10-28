using System.Globalization;
using System.Text;
using War3Net.Build.Environment;

namespace Moonwell.Build
{
    /// <summary>
    /// Writes the map's ground, read from <c>war3map.w3e</c>, as the C# file <c>Moonwell.Terrain</c> reads its grid from. Each corner
    /// is a 24-bit number written as four characters (see <c>HeightGrid</c> in the package, which reads them). A
    /// water-flagged corner takes the water's surface only where that is above its ground; elsewhere it takes the ground
    /// and its water bit is clear. (A water-flagged corner in the map's outer rings, which the game reads as ground,
    /// is still given by this rule: those rings are outside the playable area.) A ramp-flagged corner is raised 32 for each
    /// of the (up to four) tiles it is a corner of that has, among its other three corners, a ramp-flagged corner whose
    /// cliff level is exactly one higher: a higher ramp corner along x or y (in two of the tiles) gives 64, one only
    /// diagonal (in one tile) gives 32, an L of them (three tiles) gives 96. Corners outside the grid are not counted.
    /// The raised ground is what the water rule compares the surface with and what is written, while the corner's cliff
    /// level and flags are unchanged. The rule fits all 15,625 corners of Lost Temple's playable area (10-25).
    /// </summary>
    public static class TerrainWriter
    {
        private const string Alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";

        // The height times 20 is stored with this added, in 17 bits.
        private const int ZeroHeight = 49152;

        private const int HeightLimit = 131072;

        private const string FileName = "TerrainData.g.cs";

        /// <summary>The grid's text: four characters a corner, row by row from the bottom left.</summary>
        /// <param name="environment">The map's ground.</param>
        /// <returns>The text.</returns>
        /// <exception cref="BuildException">A corner's height is outside what the grid can hold, or the map lists the wrong number of corners.</exception>
        public static string Encode(MapEnvironment environment)
        {
            var columns = Columns(environment);
            var rows = Rows(environment);
            if (environment.TerrainTiles.Count != columns * rows)
            {
                throw new BuildException($"war3map.w3e lists {environment.TerrainTiles.Count} corners, but a map of {environment.Width} by {environment.Height} tiles has {columns} by {rows}");
            }

            var text = new StringBuilder(columns * rows * 4);
            for (var index = 0; index < environment.TerrainTiles.Count; index++)
            {
                var tile = environment.TerrainTiles[index];
                var ground = Ground(environment, columns, rows, index);

                var surface = (tile.WaterHeight * 128f) - 89.6f;

                // Water counts only where its surface is above the ground; below it, the game reads the ground.
                var water = tile.IsWater && surface > ground;
                var z = water ? surface : ground;
                var h = (int)MathF.Round(z * 20f) + ZeroHeight;
                if (h < 0 || h >= HeightLimit)
                {
                    throw new BuildException(
                        $"war3map.w3e: the ground at corner ({index % columns}, {index / columns}) is {z.ToString("0.##", CultureInfo.InvariantCulture)} high, outside the -2457.6 to 4096 the terrain grid holds");
                }

                var value = (h << 7) | (tile.CliffLevel << 3) | (tile.IsRamp ? 2 : 0) | (water ? 1 : 0);
                text.Append(Alphabet[(value >> 18) & 63]);
                text.Append(Alphabet[(value >> 12) & 63]);
                text.Append(Alphabet[(value >> 6) & 63]);
                text.Append(Alphabet[value & 63]);
            }

            return text.ToString();
        }

        /// <summary>
        /// The ground's height at a point beneath any water, which the grid does not keep (it keeps the water's surface where
        /// that is above the ground): the four corners of the point's tile, each its ground raised on a ramp as
        /// <see cref="Encode"/> raises it and kept to a twentieth as the grid keeps it, blended as the grid blends them. A
        /// point outside the grid reads its edge. A placed bridge's deck stands on it.
        /// </summary>
        /// <param name="environment">The map's ground.</param>
        /// <param name="x">The point's x.</param>
        /// <param name="y">The point's y.</param>
        /// <returns>The height.</returns>
        public static float GroundBeneath(MapEnvironment environment, float x, float y)
        {
            var columns = Columns(environment);
            var rows = Rows(environment);
            var u = Math.Clamp((x - environment.Left) / 128f, 0f, columns - 1);
            var v = Math.Clamp((y - environment.Bottom) / 128f, 0f, rows - 1);
            var i = Math.Min((int)MathF.Floor(u), Math.Max(columns - 2, 0));
            var j = Math.Min((int)MathF.Floor(v), Math.Max(rows - 2, 0));
            var right = i + 1 < columns ? i + 1 : i;
            var top = j + 1 < rows ? j + 1 : j;
            var fx = u - i;
            var fy = v - j;
            var z00 = Kept(Ground(environment, columns, rows, (j * columns) + i));
            var z10 = Kept(Ground(environment, columns, rows, (j * columns) + right));
            var z01 = Kept(Ground(environment, columns, rows, (top * columns) + i));
            var z11 = Kept(Ground(environment, columns, rows, (top * columns) + right));
            return ((1f - fy) * (((1f - fx) * z00) + (fx * z10))) + (fy * (((1f - fx) * z01) + (fx * z11)));
        }

        /// <summary>
        /// Writes <c>TerrainData.g.cs</c> into the project's generated folder: <c>Moonwell.Generated.TerrainData</c> with the grid's
        /// size and origin as constants and its text from <c>Corners()</c>, a method so that CSharp.lua writes the text once.
        /// </summary>
        /// <param name="project">The map project.</param>
        /// <param name="environment">The map's ground.</param>
        /// <returns>The path of the written file.</returns>
        /// <exception cref="BuildException">See <see cref="Encode"/>.</exception>
        public static string Write(MapProject project, MapEnvironment environment)
        {
            var corners = Encode(environment);
            Directory.CreateDirectory(project.GeneratedFolder);
            var path = Path.Combine(project.GeneratedFolder, FileName);
            var text = string.Join(
                "\n",
                "// Generated by Moonwell's build from war3map.w3e. Do not edit; every build writes it anew.",
                "namespace Moonwell.Generated",
                "{",
                "    public static class TerrainData",
                "    {",
                "        public const int Columns = " + Columns(environment).ToString(CultureInfo.InvariantCulture) + ";",
                "        public const int Rows = " + Rows(environment).ToString(CultureInfo.InvariantCulture) + ";",
                "        public const float Left = " + Literal(environment.Left) + ";",
                "        public const float Bottom = " + Literal(environment.Bottom) + ";",
                "        public static string Corners() => \"" + corners + "\";",
                "    }",
                "}",
                string.Empty);
            File.WriteAllText(path, text);
            return path;
        }

        /// <summary>Removes the file an earlier build wrote, which a map that no longer references the package would not compile.</summary>
        /// <param name="project">The map project.</param>
        public static void Delete(MapProject project)
        {
            var path = Path.Combine(project.GeneratedFolder, FileName);
            if (File.Exists(path))
            {
                File.Delete(path);
            }
        }

        // A corner's ground: its height and cliff level, raised on a ramp, whatever water is above it.
        private static float Ground(MapEnvironment environment, int columns, int rows, int index)
        {
            var tile = environment.TerrainTiles[index];
            var ground = (tile.Height * 128f) + ((tile.CliffLevel - 2) * 128f);
            return ground + (32f * HigherRampTiles(environment, columns, rows, index));
        }

        // A height kept to a twentieth, as the grid keeps it.
        private static float Kept(float z)
        {
            return MathF.Round(z * 20f) / 20f;
        }

        // A ramp corner is raised 32 for each tile around it (up to four) that has another corner with the ramp flag and
        // a cliff level exactly one higher. A corner outside the grid is not there to be that corner.
        private static int HigherRampTiles(MapEnvironment environment, int columns, int rows, int index)
        {
            var tile = environment.TerrainTiles[index];
            if (!tile.IsRamp)
            {
                return 0;
            }

            var column = index % columns;
            var row = index / columns;
            var count = 0;
            for (var tileRow = row - 1; tileRow <= row; tileRow++)
            {
                for (var tileColumn = column - 1; tileColumn <= column; tileColumn++)
                {
                    if (TileHasHigherRamp(environment, tile, column, row, tileColumn, tileRow, columns, rows))
                    {
                        count++;
                    }
                }
            }

            return count;
        }

        // The tile whose bottom-left corner is (tileColumn, tileRow): does one of its other corners read as a higher ramp?
        private static bool TileHasHigherRamp(MapEnvironment environment, TerrainTile tile, int column, int row, int tileColumn, int tileRow, int columns, int rows)
        {
            for (var y = tileRow; y <= tileRow + 1; y++)
            {
                for (var x = tileColumn; x <= tileColumn + 1; x++)
                {
                    if ((x == column && y == row) || x < 0 || x >= columns || y < 0 || y >= rows)
                    {
                        continue;
                    }

                    var other = environment.TerrainTiles[(y * columns) + x];
                    if (other.IsRamp && other.CliffLevel == tile.CliffLevel + 1)
                    {
                        return true;
                    }
                }
            }

            return false;
        }

        // War3Net's Width and Height count tiles; the grid counts the corners between and around them, one more each way.
        private static int Columns(MapEnvironment environment)
        {
            return (int)environment.Width + 1;
        }

        private static int Rows(MapEnvironment environment)
        {
            return (int)environment.Height + 1;
        }

        private static string Literal(float value)
        {
            return value.ToString("R", CultureInfo.InvariantCulture) + "f";
        }
    }
}
