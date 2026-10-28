using System.Text;
using Moonwell.MapFiles;
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

        [Fact]
        public void a_standard_list_entry_with_a_space_or_no_object_passes_through_a_changed_list()
        {
            // The Hydra's abilities are "Aspo, Aspt", with a space; a tavern hero requires TALT, which is no object.
            var staged = Build("""
                namespace Sample
                {
                    [Moonwell.Objects.ObjectId("n001")]
                    public sealed class BigHydra : Moonwell.Objects.Creeps.Units.Hydra
                    {
                        public BigHydra()
                        {
                            Abilities.Add<Moonwell.Objects.Human.Abilities.Defend>();
                        }
                    }

                    [Moonwell.Objects.ObjectId("N002")]
                    public sealed class Brewer : Moonwell.Objects.Creeps.Units.Alchemist
                    {
                        public Brewer()
                        {
                            Requires.Add<Moonwell.Objects.Human.Units.Footman>();
                        }
                    }
                }
                """);

            var units = ReadUnits(staged);
            Assert.Equal(new[] { "0=Aspo,Aspt,Adef" }, LevelsOf(units, "n001", "uabi"));
            Assert.Equal(new[] { "0=TALT,hfoo" }, LevelsOf(units, "N002", "ureq"));
        }

        [Fact]
        public void a_field_the_metadata_lists_by_the_abilitys_own_id_is_written()
        {
            // Cargo Capacity (Car1) lists Sch2 to Sch5, not their code; a Goblin Zeppelin's hold of a map's own seats six.
            var staged = Build("""
                namespace Sample
                {
                    [Moonwell.Objects.ObjectId("A001")]
                    public sealed class SixSeats : Moonwell.Objects.Other.Abilities.CargoHold
                    {
                        public SixSeats()
                        {
                            CargoCapacity = 6;
                        }
                    }
                }
                """);

            Assert.Equal(new[] { "1=6" }, LevelsOf(ReadAbilities(staged), "A001", "Car1"));
        }

        [Fact]
        public void a_level_past_the_standard_level_count_is_written_whatever_its_value()
        {
            // Storm Bolt has 3 levels; the game gives levels 4 to 6 of a 6-level copy 75 mana unless the map writes them.
            var staged = Build("""
                namespace Sample
                {
                    [Moonwell.Objects.ObjectId("A002")]
                    public sealed class SixBolt : Moonwell.Objects.Human.Abilities.StormBolt
                    {
                        public SixBolt()
                        {
                            LevelCount = 6;
                            ManaCost = Levels(75, 75, 75, 0, 0, 0);
                            Damage = Levels(100f, 200f, 310f, 325f, 325f, 325f);
                        }
                    }

                    [Moonwell.Objects.ObjectId("A003")]
                    public sealed class FiveBolt : Moonwell.Objects.Human.Abilities.StormBolt
                    {
                        public FiveBolt()
                        {
                            LevelCount = 5;
                            Set("Htb1", 5, 325f);
                            Set("Htb1", 2, 200f);
                        }
                    }
                }
                """);

            var abilities = ReadAbilities(staged);
            Assert.Equal(new[] { "1=75", "2=75", "3=75", "4=0", "5=0", "6=0" }, LevelsOf(abilities, "A002", "amcs"));
            Assert.Equal(new[] { "1=100", "2=200", "3=310", "4=325", "5=325", "6=325" }, LevelsOf(abilities, "A002", "Htb1"));
            Assert.Equal(new[] { "5=325" }, LevelsOf(abilities, "A003", "Htb1"));
        }

        [Fact]
        public void the_placement_fields_take_world_editors_names_and_write_the_games()
        {
            // Decision 70: World Editor's "Placement Requires: Buildable, Unblighted" is the game's preventPlace "blighted,unbuildable";
            // its "Placement Prevented By: Unblighted" is the game's requirePlace "blighted", an undead building's.
            var staged = Build("""
                using Moonwell.Objects;

                namespace Sample
                {
                    [ObjectId("h001")]
                    public sealed class BlightBarracks : Human.Units.Barracks
                    {
                        public BlightBarracks()
                        {
                            PlacementRequires = PathingRequire.Buildable | PathingRequire.Unblighted;
                            PlacementPreventedBy = PathingPrevent.Unblighted;
                        }
                    }
                }
                """);

            var units = ReadUnits(staged);
            Assert.Equal(new[] { "0=blighted,unbuildable" }, LevelsOf(units, "h001", "upap"));
            Assert.Equal(new[] { "0=blighted" }, LevelsOf(units, "h001", "upar"));
        }

        [Theory]
        [InlineData("Cooldown = Levels(5f);", "acdn is given 1 level value, and the object has 3 levels")]
        [InlineData("Damage = Levels(1f, 2f, 3f, 4f);", "Htb1 is given 4 level values, and the object has 3 levels")]
        public void levels_of_another_count_than_the_objects_fail_the_build(string statement, string expected)
        {
            // Levels(5f) on a Storm Bolt of 3 levels used to change level 1 only; one value for every level is Cooldown = 5f.
            var copy = folders.TemporaryCopyOfSample();
            File.WriteAllText(Path.Combine(copy, "Objects.cs"), $$"""
                namespace Sample
                {
                    [Moonwell.Objects.ObjectId("A001")]
                    public sealed class Slow : Moonwell.Objects.Human.Abilities.StormBolt
                    {
                        public Slow()
                        {
                            {{statement}}
                        }
                    }
                }
                """);

            var error = Assert.Throws<BuildException>(() => MapBuild.Run(copy, new BuildOptions(), TextWriter.Null));

            Assert.Equal(
                $"Sample.Slow: {expected}: give one value for each level, or a single value (not in Levels) for every level",
                error.Message);
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
            var one = Assert.Single(objects, candidate => candidate.NewId == Rawcodes.IdOf(id));
            return one.Modifications
                .Where(modification => modification.Id == Rawcodes.IdOf(field))
                .OrderBy(modification => modification.Level)
                .Select(modification => $"{modification.Level}={modification.Value}")
                .ToArray();
        }

        private static IReadOnlyList<SimpleObjectModification> ReadUnits(string staged)
        {
            using var reader = new BinaryReader(File.OpenRead(Path.Combine(staged, "war3map.w3u")), Encoding.UTF8);
            return reader.ReadUnitObjectData().NewUnits;
        }

        // A field of an object without levels, as "0=value".
        private static string[] LevelsOf(IEnumerable<SimpleObjectModification> objects, string id, string field)
        {
            var one = Assert.Single(objects, candidate => candidate.NewId == Rawcodes.IdOf(id));
            return one.Modifications
                .Where(modification => modification.Id == Rawcodes.IdOf(field))
                .Select(modification => $"0={modification.Value}")
                .ToArray();
        }

        private static IReadOnlyList<LevelObjectModification> ReadAbilities(string staged)
        {
            using var reader = new BinaryReader(File.OpenRead(Path.Combine(staged, "war3map.w3a")), Encoding.UTF8);
            return reader.ReadAbilityObjectData().NewAbilities;
        }

        private static IReadOnlyList<LevelObjectModification> ReadUpgrades(string staged)
        {
            using var reader = new BinaryReader(File.OpenRead(Path.Combine(staged, "war3map.w3q")), Encoding.UTF8);
            return reader.ReadUpgradeObjectData().NewUpgrades;
        }
    }
}
