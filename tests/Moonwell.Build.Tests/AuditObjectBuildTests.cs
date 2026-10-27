using System.Text;
using War3Net.Build.Extensions;
using War3Net.Build.Object;
using Xunit;

namespace Moonwell.Build.Tests
{
    /// <summary>The objects a build writes after the full audit's object fixes: each case builds a copy of the Sample with its own objects.</summary>
    [Collection("Sample")]
    public sealed class AuditObjectBuildTests : IDisposable
    {
        private readonly TestFolders folders;

        public AuditObjectBuildTests(ITestOutputHelper output)
        {
            folders = new TestFolders(output);
        }

        public void Dispose()
        {
            folders.Dispose();
        }

        [Fact]
        public void a_list_that_differs_per_level_takes_the_changes_on_each_levels_own_list()
        {
            // Decision 72: Iron Forged Swords needs nothing, then a Keep, then a Castle; the copy keeps both tiers.
            var staged = Build("""
                namespace Sample
                {
                    [Moonwell.Objects.ObjectId("R001")]
                    public sealed class SteelSwords : Moonwell.Objects.Human.Upgrades.IronForgedSwords
                    {
                        public SteelSwords()
                        {
                            Requires.Add<Moonwell.Objects.Human.Units.Blacksmith>();
                        }
                    }

                    [Moonwell.Objects.ObjectId("R002")]
                    public sealed class NoKeep : Moonwell.Objects.Human.Upgrades.IronPlating
                    {
                        public NoKeep()
                        {
                            Requires.Remove<Moonwell.Objects.Human.Units.Keep>();
                        }
                    }

                    [Moonwell.Objects.ObjectId("R003")]
                    public sealed class BarracksSwords : Moonwell.Objects.Human.Upgrades.IronForgedSwords
                    {
                        public BarracksSwords()
                        {
                            Requires.Replace<Moonwell.Objects.Human.Units.Keep, Moonwell.Objects.Human.Units.Barracks>();
                        }
                    }

                    [Moonwell.Objects.ObjectId("R004")]
                    public sealed class SmithySwords : Moonwell.Objects.Human.Upgrades.IronForgedSwords
                    {
                        public SmithySwords()
                        {
                            Requires.Set<Moonwell.Objects.Human.Units.Blacksmith>();
                        }
                    }
                }
                """);

            var upgrades = ReadUpgrades(staged);
            Assert.Equal(new[] { "1=hbla", "2=hkee,hbla", "3=hcas,hbla" }, LevelsOf(upgrades, "R001", "greq"));
            Assert.Equal(new[] { "1=", "2=", "3=hcas" }, LevelsOf(upgrades, "R002", "greq"));
            Assert.Equal(new[] { "1=", "2=hbar", "3=hcas" }, LevelsOf(upgrades, "R003", "greq"));
            Assert.Equal(new[] { "1=hbla", "2=hbla", "3=hbla" }, LevelsOf(upgrades, "R004", "greq"));
        }

        private string Build(string objects)
        {
            var copy = folders.TemporaryCopyOfSample();
            File.WriteAllText(Path.Combine(copy, "Objects.cs"), objects);
            MapBuild.Run(copy, new BuildOptions(), TextWriter.Null);
            return Path.Combine(copy, "build", "Sample");
        }

        // Each level's value of a field of an object, as "level=value", by level.
        private static string[] LevelsOf(IEnumerable<LevelObjectModification> objects, string id, string field)
        {
            var one = Assert.Single(objects, candidate => candidate.NewId == ObjectTable.IdOf(id));
            return one.Modifications
                .Where(modification => modification.Id == ObjectTable.IdOf(field))
                .OrderBy(modification => modification.Level)
                .Select(modification => $"{modification.Level}={modification.Value}")
                .ToArray();
        }

        private static IReadOnlyList<LevelObjectModification> ReadUpgrades(string staged)
        {
            using var reader = new BinaryReader(File.OpenRead(Path.Combine(staged, "war3map.w3q")), Encoding.UTF8);
            return reader.ReadUpgradeObjectData().NewUpgrades;
        }
    }
}
