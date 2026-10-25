using System.Globalization;
using War3Net.Build.Environment;
using War3Net.Build.Extensions;
using Xunit;

namespace Moonwell.Build.Tests
{
    public sealed class TerrainWriterTests : IDisposable
    {
        private readonly TestFolders folders = new TestFolders();

        public void Dispose()
        {
            folders.Dispose();
        }

        [Fact]
        public void encodes_the_pinned_corners()
        {
            var environment = MakeEnvironment(1, 1);
            environment.TerrainTiles.Add(new TerrainTile { Height = 0, CliffLevel = 2 });
            environment.TerrainTiles.Add(new TerrainTile { Flags = TileFlags.Water, Height = -16, WaterHeight = 0, CliffLevel = 2 });
            environment.TerrainTiles.Add(new TerrainTile { Height = 0, CliffLevel = 3, Flags = TileFlags.Ramp });
            environment.TerrainTiles.Add(new TerrainTile { Height = -16, CliffLevel = 0 });

            Assert.Equal("YAAQXIARZQAaBgAA", TerrainWriter.Encode(environment));
        }

        [Fact]
        public void shallow_water_reads_the_ground()
        {
            var environment = MakeEnvironment(0, 0);
            environment.TerrainTiles.Add(new TerrainTile { Flags = TileFlags.Water, Height = 0, WaterHeight = 0, CliffLevel = 2 });

            Assert.Equal("YAAQ", TerrainWriter.Encode(environment));
        }

        [Fact]
        public void largest_map_fits()
        {
            var environment = MakeEnvironment(480, 480);
            for (var i = 0; i < 481 * 481; i++)
            {
                environment.TerrainTiles.Add(new TerrainTile { Height = 0, CliffLevel = 2 });
            }

            var text = TerrainWriter.Encode(environment);

            Assert.Equal(925444, text.Length);
            Assert.True(text.Length < 1048576);
        }

        [Fact]
        public void out_of_range_fails()
        {
            var environment = MakeEnvironment(1, 0);
            environment.TerrainTiles.Add(new TerrainTile { Height = 0, CliffLevel = 2 });
            environment.TerrainTiles.Add(new TerrainTile { Height = 100, CliffLevel = 2 });

            var error = Assert.Throws<BuildException>(() => TerrainWriter.Encode(environment));

            Assert.Contains("(1, 0)", error.Message);
        }

        [Fact]
        public void writes_the_data_file_with_the_invariant_culture()
        {
            var project = MapProject.Find(folders.TemporaryCopyOfSample());
            var environment = MakeEnvironment(0, 0);
            environment.Left = -2560.5f;
            environment.Bottom = -3072;
            environment.TerrainTiles.Add(new TerrainTile { Height = 0, CliffLevel = 2 });
            var previous = CultureInfo.CurrentCulture;
            CultureInfo.CurrentCulture = new CultureInfo("sv-SE");
            string text;
            try
            {
                text = File.ReadAllText(TerrainWriter.Write(project, environment));
            }
            finally
            {
                CultureInfo.CurrentCulture = previous;
            }

            Assert.Contains("namespace Moonwell.Generated", text);
            Assert.Contains("public const int Columns = 1;", text);
            Assert.Contains("public const int Rows = 1;", text);
            Assert.Contains("public const float Left = -2560.5f;", text);
            Assert.Contains("public const float Bottom = -3072f;", text);
            Assert.Contains("public static string Corners() => \"YAAQ\";", text);

            TerrainWriter.Delete(project);
            Assert.False(File.Exists(Path.Combine(project.GeneratedFolder, "TerrainData.g.cs")));
            TerrainWriter.Delete(project);
        }

        [Fact]
        public void entry_installs_in_order()
        {
            var project = MapProject.Find(folders.TemporaryCopyOfSample());
            var settings = new MapSettings("Sample.SampleMap", "n", "a", "d", "Any");
            var installs = new[]
            {
                "global::Moonwell.DamageSystem.Install();",
                "global::Moonwell.Terrain.Install(1, 1, 0f, 0f, \"YAAQ\");",
            };

            var text = File.ReadAllText(EntryWriter.Write(project, settings, true, installs));

            var damage = text.IndexOf("DamageSystem.Install();", StringComparison.Ordinal);
            var terrain = text.IndexOf("Terrain.Install(", StringComparison.Ordinal);
            var register = text.IndexOf("ObjectRegistry.Register();", StringComparison.Ordinal);
            Assert.True(damage >= 0 && damage < terrain && terrain < register, text);
        }

        [Fact]
        public void a_real_map_has_one_more_corner_than_tiles_each_way()
        {
            var path = Path.GetFullPath(Path.Combine(AppContext.BaseDirectory, "..", "..", "..", "..", "..", "template", "MoonwellMap", "map", "war3map.w3e"));
            MapEnvironment environment;
            using (var reader = new BinaryReader(File.OpenRead(path)))
            {
                environment = reader.ReadMapEnvironment();
            }

            Assert.Equal((int)((environment.Width + 1) * (environment.Height + 1)), environment.TerrainTiles.Count);
            Assert.Equal(environment.TerrainTiles.Count * 4, TerrainWriter.Encode(environment).Length);
        }

        [Fact]
        public void a_ramp_corner_below_a_higher_ramp_corner_is_raised_half_a_level()
        {
            var read = MakeEnvironment(2, 0);
            read.TerrainTiles.Add(Ramp(0, 2));
            read.TerrainTiles.Add(Ramp(0, 3));
            read.TerrainTiles.Add(Ramp(0, 3));

            var text = TerrainWriter.Encode(read);

            // z 64, level 2, ramp; then z 128, level 3, ramp twice (the pinned "ZQAa").
            Assert.Equal("YoASZQAaZQAa", text);
        }

        [Fact]
        public void a_ramp_corner_without_a_ramp_neighbour_one_level_up_is_not_raised()
        {
            var noRampFlag = MakeEnvironment(2, 0);
            noRampFlag.TerrainTiles.Add(Ramp(0, 2));
            noRampFlag.TerrainTiles.Add(new TerrainTile { Height = 0, CliffLevel = 3 });
            noRampFlag.TerrainTiles.Add(Ramp(0, 3));
            Assert.StartsWith("YAAS", TerrainWriter.Encode(noRampFlag));

            var twoUp = MakeEnvironment(1, 0);
            twoUp.TerrainTiles.Add(Ramp(0, 2));
            twoUp.TerrainTiles.Add(Ramp(0, 4));
            Assert.StartsWith("YAAS", TerrainWriter.Encode(twoUp));
        }

        [Fact]
        public void a_ramp_corner_with_a_higher_ramp_corner_only_diagonal_is_raised_a_quarter_level()
        {
            var read = MakeEnvironment(1, 1);
            read.TerrainTiles.Add(Ramp(0, 2));
            read.TerrainTiles.Add(Ramp(0, 2));
            read.TerrainTiles.Add(Ramp(0, 2));
            read.TerrainTiles.Add(Ramp(0, 3));

            var text = TerrainWriter.Encode(read);

            // The bottom-left corner: z 32, level 2, ramp.
            Assert.StartsWith("YUAS", text);
        }

        [Fact]
        public void a_ramp_corner_in_an_l_of_higher_ramp_corners_is_raised_three_quarters()
        {
            var read = MakeEnvironment(2, 2);
            for (var row = 0; row < 3; row++)
            {
                for (var column = 0; column < 3; column++)
                {
                    var higher = (column == 2 && row == 1) || (column == 1 && row == 2) || (column == 2 && row == 2);
                    read.TerrainTiles.Add(Ramp(0, higher ? 3 : 2));
                }
            }

            var text = TerrainWriter.Encode(read);

            // The centre (index 4): z 96, level 2, ramp.
            Assert.Equal("Y8AS", text.Substring(4 * 4, 4));
        }

        [Fact]
        public void raising_runs_along_rows_too()
        {
            var read = MakeEnvironment(0, 2);
            read.TerrainTiles.Add(Ramp(0, 2));
            read.TerrainTiles.Add(Ramp(0, 3));
            read.TerrainTiles.Add(Ramp(0, 3));

            var text = TerrainWriter.Encode(read);

            // z 64, level 2, ramp; then z 128, level 3, ramp twice.
            Assert.Equal("YoASZQAaZQAa", text);
        }

        private static TerrainTile Ramp(float height, int cliffLevel)
        {
            return new TerrainTile { Height = height, CliffLevel = cliffLevel, Flags = TileFlags.Ramp };
        }

        private static MapEnvironment MakeEnvironment(uint tilesAcross, uint tilesUp)
        {
            return new MapEnvironment(MapEnvironmentFormatVersion.v11) { Width = tilesAcross, Height = tilesUp };
        }
    }
}
