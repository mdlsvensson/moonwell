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
            environment.TerrainTiles.Add(new TerrainTile { Flags = TileFlags.Water, WaterHeight = 0, CliffLevel = 2 });
            environment.TerrainTiles.Add(new TerrainTile { Height = 0, CliffLevel = 3, Flags = TileFlags.Ramp });
            environment.TerrainTiles.Add(new TerrainTile { Height = -16, CliffLevel = 0 });

            Assert.Equal("YAAQXIARZQAaBgAA", TerrainWriter.Encode(environment));
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

        private static MapEnvironment MakeEnvironment(uint tilesAcross, uint tilesUp)
        {
            return new MapEnvironment(MapEnvironmentFormatVersion.v11) { Width = tilesAcross, Height = tilesUp };
        }
    }
}
