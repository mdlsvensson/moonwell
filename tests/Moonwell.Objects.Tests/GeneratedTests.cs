using System.Reflection;
using System.Text.Json;

namespace Moonwell.Objects.Tests
{
    /// <summary>A generated hero with the protected members of its kind base opened, as a modder's class has them.</summary>
    public class OpenPaladin : Moonwell.Objects.Human.Units.Paladin
    {
        public void Put(string field, object value)
        {
            Set(field, value);
        }

        public void PutLevel(string field, int level, object value)
        {
            Set(field, level, value);
        }

        public void SetMaxLife(int value)
        {
            MaxLife = value;
        }
    }

    /// <summary>A generated ability with the protected members of its kind base opened.</summary>
    public class OpenStormBolt : Moonwell.Objects.Human.Abilities.StormBolt
    {
        public void Put(string field, object value)
        {
            Set(field, value);
        }

        public void PutLevel(string field, int level, object value)
        {
            Set(field, level, value);
        }

        public Levels<float> PerLevelOf(float first, float second)
        {
            return Levels(first, second);
        }
    }

    // A hero's MaxLife reads StandardValues.HeroBonus, a static the tests of the collection set.
    [Collection(StandardValuesCollection.Name)]
    public class GeneratedTests
    {
        private static readonly Assembly Objects = typeof(StandardAttribute).Assembly;

        private static List<Type> StandardClasses()
        {
            var classes = new List<Type>();
            foreach (var type in Objects.GetTypes())
            {
                if (type.GetCustomAttribute<StandardAttribute>(false) != null)
                {
                    classes.Add(type);
                }
            }

            return classes;
        }

        private static int CountIn(string kind)
        {
            var count = 0;
            foreach (var type in StandardClasses())
            {
                if (type.DeclaringType!.Name == kind)
                {
                    count++;
                }
            }

            return count;
        }

        private static string GeneratedFolder()
        {
            for (var folder = new DirectoryInfo(AppContext.BaseDirectory); folder != null; folder = folder.Parent)
            {
                var generated = Path.Combine(folder.FullName, "src", "Moonwell.Objects", "Generated");
                if (Directory.Exists(generated))
                {
                    return generated;
                }
            }

            throw new InvalidOperationException("src/Moonwell.Objects/Generated is not above " + AppContext.BaseDirectory);
        }

        [Fact]
        public void paladin_exists()
        {
            var paladin = Objects.GetType("Moonwell.Objects.Human+Units+Paladin");

            Assert.NotNull(paladin);
            Assert.Equal("Hpal", paladin!.GetCustomAttribute<StandardAttribute>(false)!.Id);
            Assert.Equal("StandardHero", paladin.BaseType!.Name);
            Assert.Equal("Moonwell.Objects", paladin.BaseType.Namespace);
            Assert.True(paladin.IsAssignableTo(typeof(Moonwell.Hero)));
        }

        [Fact]
        public void a_modder_class_inherits_a_standard_hero()
        {
            var arthas = new OpenPaladin();
            arthas.SetMaxLife(1300);

            // A hero's life set by its live name is its total, kept apart from the base field.
            Assert.Equal(1300, arthas.Totals["uhpm"]);
            Assert.Empty(arthas.Declared);
            Assert.Equal(1300, arthas.MaxLife);
        }

        [Fact]
        public void storm_bolt_damage_is_per_level()
        {
            var stormBolt = Objects.GetType("Moonwell.Objects.Human+Abilities+StormBolt")!;

            var damage = stormBolt.GetProperties(BindingFlags.Public | BindingFlags.Instance | BindingFlags.DeclaredOnly)
                .Single(p => p.GetCustomAttribute<FieldAttribute>()!.Id == "Htb1");

            Assert.Equal(typeof(Levels<float>), damage.PropertyType);
            Assert.Equal("Damage", damage.Name);
            Assert.Equal("AHtb", stormBolt.GetCustomAttribute<StandardAttribute>(false)!.Id);
            Assert.Equal("StandardAbility", stormBolt.BaseType!.Name);
        }

        [Fact]
        public void a_common_ability_field_is_on_the_kind_base()
        {
            var ability = Objects.GetType("Moonwell.Objects.StandardAbility")!;

            var cooldown = ability.GetProperties().Single(p => p.GetCustomAttribute<FieldAttribute>()?.Id == "acdn");

            Assert.Equal(typeof(Levels<float>), cooldown.PropertyType);
            Assert.Equal("Cooldown", cooldown.Name);
        }

        [Fact]
        public void every_class_is_ignored()
        {
            var checkedTypes = 0;
            foreach (var path in Directory.GetFiles(GeneratedFolder(), "*.g.cs", SearchOption.AllDirectories))
            {
                var lines = File.ReadAllLines(path);
                Assert.StartsWith("// Generated by Moonwell.Generator from the game's data, game 3.0.1.24371. Do not edit.", lines[0]);
                for (var i = 0; i < lines.Length; i++)
                {
                    var line = lines[i].TrimStart();
                    if (!line.StartsWith("public class ", StringComparison.Ordinal) && !line.StartsWith("public abstract class ", StringComparison.Ordinal))
                    {
                        continue;
                    }

                    var above = i - 1;
                    while (lines[above].TrimStart().StartsWith("[", StringComparison.Ordinal))
                    {
                        above--;
                    }

                    Assert.True(lines[above].Trim() == "/// @CSharpLua.Ignore", $"{Path.GetFileName(path)} line {i + 1}: {line} is not ignored");
                    checkedTypes++;
                }
            }

            Assert.Equal(StandardClasses().Count + 8, checkedTypes);
        }

        [Fact]
        public void counts()
        {
            Assert.InRange(CountIn("Units"), 900, 1000);
            Assert.InRange(CountIn("Abilities"), 1500, 1600);
            Assert.InRange(CountIn("Items"), 600, 700);
            Assert.InRange(CountIn("Destructables"), 300, 400);
            Assert.InRange(CountIn("Doodads"), 700, 800);
            Assert.InRange(CountIn("Buffs"), 300, 350);
            Assert.InRange(CountIn("Upgrades"), 80, 100);
        }

        [Fact]
        public void no_class_is_named_after_a_world_editor_string_key()
        {
            foreach (var type in StandardClasses())
            {
                Assert.False(type.Name.StartsWith("WESTRING", StringComparison.OrdinalIgnoreCase), type.FullName);
            }
        }

        [Fact]
        public void each_class_has_the_base_of_its_kind()
        {
            var ids = new HashSet<string>();
            foreach (var type in StandardClasses())
            {
                var id = type.GetCustomAttribute<StandardAttribute>(false)!.Id;
                var kind = type.DeclaringType!.Name;
                var expected = kind switch
                {
                    "Units" => char.IsAsciiLetterUpper(id[0]) ? "StandardHero" : "StandardUnit",
                    "Abilities" => "StandardAbility",
                    "Items" => "StandardItem",
                    "Destructables" => "StandardDestructable",
                    "Doodads" => "StandardDoodad",
                    "Buffs" => "StandardBuff",
                    "Upgrades" => "StandardUpgrade",
                    _ => throw new InvalidOperationException(type.FullName),
                };

                Assert.True(expected == type.BaseType!.Name, $"{type.FullName} is {id}: base {type.BaseType.Name}, not {expected}");
                Assert.True(ids.Add(kind + ":" + id), $"{kind} {id} has two classes");
            }
        }

        [Fact]
        public void every_class_can_be_made_and_has_nothing_declared()
        {
            foreach (var type in StandardClasses())
            {
                var made = Activator.CreateInstance(type)!;

                var declared = (IReadOnlyDictionary<string, object?>)type.GetProperty("Declared", BindingFlags.Instance | BindingFlags.NonPublic)!.GetValue(made)!;
                Assert.Empty(declared);
            }
        }

        [Fact]
        public void set_by_field_id()
        {
            var arthas = new OpenPaladin();
            arthas.Put("uhpm", 800);

            Assert.Equal(800, arthas.MaxLife);
            Assert.Equal(800, arthas.Declared["uhpm"]);
        }

        [Fact]
        public void set_of_one_level_by_field_id()
        {
            var bolt = new OpenStormBolt();
            bolt.PutLevel("acdn", 2, 5f);
            bolt.Put("Htb1", bolt.PerLevelOf(10f, 20f));

            Assert.Equal(5f, bolt.Declared["acdn:2"]);
            Assert.Equal(20f, bolt.Damage[2]);
        }

        [Fact]
        public void set_with_the_wrong_type_names_the_field()
        {
            var arthas = new OpenPaladin();

            var error = Assert.Throws<ArgumentException>(() => arthas.Put("uhpm", 800.5f));

            Assert.StartsWith("StandardHero field uhpm holds int, not float", error.Message);
        }

        [Fact]
        public void set_of_a_per_level_field_needs_levels_or_a_level()
        {
            var bolt = new OpenStormBolt();

            var whole = Assert.Throws<ArgumentException>(() => bolt.Put("acdn", 5f));
            var mistyped = Assert.Throws<ArgumentException>(() => bolt.PutLevel("acdn", 1, 5));
            var early = Assert.Throws<ArgumentOutOfRangeException>(() => bolt.PutLevel("acdn", 0, 5f));
            var unleveled = Assert.Throws<ArgumentException>(() => new OpenPaladin().PutLevel("uhpm", 1, 5));

            Assert.StartsWith("StandardAbility field acdn holds Levels<float>, not float; give the value of one level", whole.Message);
            Assert.StartsWith("StandardAbility field acdn holds float for each level, not int", mistyped.Message);
            Assert.StartsWith("StandardAbility field acdn: levels count from 1", early.Message);
            Assert.StartsWith("StandardHero field uhpm holds int, which has no levels", unleveled.Message);
        }

        [Fact]
        public void set_of_an_unknown_field_names_it()
        {
            var arthas = new OpenPaladin();

            var error = Assert.Throws<ArgumentException>(() => arthas.Put("zzzz", 1));

            Assert.StartsWith("StandardHero has no field zzzz", error.Message);
        }

        [Fact]
        public void set_of_another_abilitys_field_names_the_class_and_the_field()
        {
            var bolt = new OpenStormBolt();

            var whole = Assert.Throws<ArgumentException>(() => bolt.Put("Hhb1", bolt.PerLevelOf(10f, 20f)));
            var level = Assert.Throws<ArgumentException>(() => bolt.PutLevel("Hhb1", 1, 10f));

            Assert.StartsWith("StormBolt has no field Hhb1, which other abilities have", whole.Message);
            Assert.StartsWith("StormBolt has no field Hhb1, which other abilities have", level.Message);
            Assert.Empty(bolt.Declared);
        }

        [Fact]
        public void the_values_are_embedded_for_each_kind()
        {
            foreach (var kind in new[] { "unit", "item", "destructable", "doodad", "ability", "buff", "upgrade" })
            {
                using var stream = Objects.GetManifestResourceStream($"Moonwell.Objects.Values.{kind}.json");
                Assert.NotNull(stream);
                using var document = JsonDocument.Parse(stream!);
                Assert.True(document.RootElement.GetProperty("fields").GetArrayLength() > 0);
                Assert.True(document.RootElement.GetProperty("objects").EnumerateObject().Any());
            }
        }

        [Fact]
        public void the_unit_values_hold_the_paladin_and_its_sets()
        {
            using var stream = Objects.GetManifestResourceStream("Moonwell.Objects.Values.unit.json")!;
            using var document = JsonDocument.Parse(stream);

            var paladin = document.RootElement.GetProperty("objects").GetProperty("Hpal");

            Assert.Equal("Paladin", paladin.GetProperty("values").GetProperty("unam").GetString());
            Assert.True(int.Parse(paladin.GetProperty("values").GetProperty("uhpm").GetString()!) > 0);
            Assert.Equal(JsonValueKind.Object, paladin.GetProperty("sets").ValueKind);
        }
    }
}
