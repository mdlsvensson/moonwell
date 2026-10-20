using System.Reflection;
using System.Text.Json;

namespace Moonwell.Objects.Tests
{
    /// <summary>A Paladin of a map's own that sets nothing.</summary>
    public class Squire : Moonwell.Objects.Human.Units.Paladin
    {
    }

    /// <summary>A Footman of a map's own that sets its hit points.</summary>
    public class Sentinel : Moonwell.Objects.Human.Units.Footman
    {
        public Sentinel()
        {
            HitPoints = 900;
        }
    }

    /// <summary>A Phoenix of a map's own: the Phoenix's standard regeneration, -25, is a negative where World Editor allows none.</summary>
    public class Firebird : Moonwell.Objects.Human.Units.Phoenix
    {
        public Firebird()
        {
            Name = "Firebird";
        }
    }

    /// <summary>A Phoenix of a map's own that puts the Phoenix's own regeneration itself.</summary>
    public class ColdFirebird : Moonwell.Objects.Human.Units.Phoenix
    {
        public ColdFirebird()
        {
            HitPointRegen = -25f;
        }
    }

    /// <summary>A Druid of the Talon of a map's own that sets nothing.</summary>
    public class Talon : Moonwell.Objects.NightElf.Units.DruidOfTheTalon
    {
    }

    /// <summary>A Storm Bolt of a map's own that sets nothing.</summary>
    public class PlainBolt : Moonwell.Objects.Human.Abilities.StormBolt
    {
    }

    /// <summary>
    /// The values a generated standard class sets in its constructor: the base tables', read when no build gives the values
    /// (<see cref="StandardValues.Read"/> is null), never declared, never checked, and given way to a map's own values and to
    /// the data set's values the build reads. In one collection with <see cref="DeclaredTests"/>, which sets StandardValues.Read.
    /// </summary>
    [Collection(StandardValuesCollection.Name)]
    public class StandardClassesTests : IDisposable
    {
        private static readonly Assembly Objects = typeof(StandardAttribute).Assembly;

        private static readonly Lazy<Dictionary<string, JsonElement>> Values = new Lazy<Dictionary<string, JsonElement>>(ReadValues);

        public StandardClassesTests()
        {
            StandardValues.Read = null;
        }

        public void Dispose()
        {
            StandardValues.Read = null;
        }

        private static Dictionary<string, JsonElement> ReadValues()
        {
            var values = new Dictionary<string, JsonElement>(StringComparer.Ordinal);
            foreach (var kind in new[] { "unit", "item", "destructable", "doodad", "ability", "buff", "upgrade" })
            {
                using var stream = Objects.GetManifestResourceStream($"Moonwell.Objects.Values.{kind}.json")!;
                using var document = JsonDocument.Parse(stream);
                foreach (var entry in document.RootElement.GetProperty("objects").EnumerateObject())
                {
                    values[entry.Name] = entry.Value.GetProperty("values").Clone();
                }
            }

            return values;
        }

        private static string Base(string id, string key)
        {
            return Values.Value[id].GetProperty(key).GetString()!;
        }

        private static IReadOnlyDictionary<string, object?> DeclaredOf(object made)
        {
            for (var type = made.GetType(); type != null; type = type.BaseType)
            {
                var property = type.GetProperty("Declared", BindingFlags.Instance | BindingFlags.NonPublic | BindingFlags.Public | BindingFlags.DeclaredOnly);
                if (property != null)
                {
                    return (IReadOnlyDictionary<string, object?>)property.GetValue(made)!;
                }
            }

            throw new InvalidOperationException(made.GetType() + " has no Declared");
        }

        [Fact]
        public void the_paladins_constructor_sets_the_base_tables_hit_points()
        {
            var paladin = new Moonwell.Objects.Human.Units.Paladin();

            Assert.Equal(int.Parse(Base("Hpal", "uhpm")), paladin.HitPoints);
            Assert.Equal(100, paladin.HitPoints);
            Assert.Equal("Paladin", paladin.Name);
            Assert.Equal(AttackType.Hero, paladin.AttackType);
            Assert.Equal(
                new object[] { typeof(Moonwell.Objects.Human.Abilities.HolyLight), typeof(Moonwell.Objects.Human.Abilities.DivineShield), typeof(Moonwell.Objects.Human.Abilities.Resurrection), typeof(Moonwell.Objects.Human.Abilities.DevotionAura) },
                paladin.HeroAbilities.Entries);
        }

        [Fact]
        public void a_standard_class_and_a_class_that_sets_nothing_declare_nothing()
        {
            Assert.Empty(DeclaredOf(new Moonwell.Objects.Human.Units.Paladin()));
            Assert.Empty(DeclaredOf(new Squire()));
            Assert.Equal(300, new Squire().Speed);
        }

        [Fact]
        public void a_map_classs_own_value_wins_and_is_the_only_one_declared()
        {
            var sentinel = new Sentinel();

            Assert.Equal(900, sentinel.HitPoints);
            Assert.Equal(new[] { "uhpm" }, DeclaredOf(sentinel).Keys);
            Assert.Equal(900, DeclaredOf(sentinel)["uhpm"]);
        }

        [Fact]
        public void a_map_classs_own_value_is_checked()
        {
            var error = Assert.Throws<FieldRuleException>(() => new Giant());

            Assert.Equal("Giant.HitPoints is 600000; Hit Points Maximum (Base) (uhpm) allows 1 to 500000; [BeyondLimits] goes past it", error.Message);
        }

        [Fact]
        public void a_standard_value_outside_world_editors_limits_is_not_checked()
        {
            Assert.Equal("-25", Base("hphx", "uhpr"));

            var firebird = new Firebird();

            Assert.Equal(-25f, firebird.HitPointRegen);
            Assert.Equal(new[] { "unam" }, DeclaredOf(firebird).Keys);
        }

        [Fact]
        public void the_same_value_put_by_a_map_class_is_checked()
        {
            var error = Assert.Throws<FieldRuleException>(() => new ColdFirebird());

            Assert.StartsWith("ColdFirebird.HitPointRegen is -25; ", error.Message);
        }

        [Fact]
        public void with_values_from_a_build_an_unset_field_reads_the_data_sets_value()
        {
            // A map on melee_v0: the Paladin's speed there is 270, the base tables' (which its constructor sets) 300.
            StandardValues.Read = (id, field, level) => id == "Hpal" && field == "umvs" ? "270" : id == "hfoo" && field == "uabi" ? "Adef" : null;

            Assert.Equal(270, new Squire().Speed);
            Assert.Equal(270, new Moonwell.Objects.Human.Units.Paladin().Speed);
            // The set has no value for the Paladin's hit points: the base tables' value is not used either.
            Assert.Equal(0, new Squire().HitPoints);
            Assert.Equal(new object[] { typeof(Moonwell.Objects.Human.Abilities.Defend) }, new Sentinel().Abilities.Entries);
            Assert.Equal(new object[] { typeof(Moonwell.Objects.Human.Abilities.Defend) }, new Sentinel().Abilities.Standard);
            Assert.Empty(DeclaredOf(new Squire()));
        }

        [Fact]
        public void a_list_starts_as_the_constructors_list_and_a_change_to_it_is_the_map_classs()
        {
            var abilities = new Sentinel().Abilities;

            Assert.Equal("Adef,Aihn", Base("hfoo", "uabi"));
            Assert.Equal(new object[] { typeof(Moonwell.Objects.Human.Abilities.Defend), typeof(Moonwell.Objects.Other.Abilities.UnitInventoryAihn) }, abilities.Entries);
            Assert.Equal(abilities.Entries, abilities.Standard);
            abilities.Remove<Moonwell.Objects.Human.Abilities.Defend>();
            Assert.Equal(2, new Sentinel().Abilities.Count);
        }

        [Fact]
        public void levels_and_references_start_as_the_constructors()
        {
            var bolt = new PlainBolt();
            Assert.Equal(float.Parse(Base("AHtb", "Htb1:2"), System.Globalization.CultureInfo.InvariantCulture), bolt.Damage[2]);
            Assert.Equal(Base("AHtb", "atp1:3"), bolt.Tooltip[3]);

            Assert.True(new Talon().DefaultActiveAbility.Is<Moonwell.Objects.NightElf.Abilities.FaerieFireAfae>());
            Assert.Empty(DeclaredOf(new Talon()));
        }

        [Fact]
        public void every_standard_class_reads_each_base_value_as_the_net_side_reads_its_text()
        {
            var parse = typeof(StandardValues).GetMethod("Parse", BindingFlags.Static | BindingFlags.NonPublic)!;
            var checkedValues = 0;
            foreach (var type in Objects.GetTypes().Where(type => type.GetCustomAttribute<StandardAttribute>(false) != null))
            {
                var id = type.GetCustomAttribute<StandardAttribute>(false)!.Id;
                var made = Activator.CreateInstance(type)!;
                Assert.Empty(DeclaredOf(made));
                var properties = new Dictionary<string, PropertyInfo>(StringComparer.Ordinal);
                for (var chain = type; chain != null; chain = chain.BaseType)
                {
                    foreach (var property in chain.GetProperties(BindingFlags.Instance | BindingFlags.Public | BindingFlags.DeclaredOnly))
                    {
                        if (property.GetCustomAttribute<FieldAttribute>(false) is { } field)
                        {
                            properties.TryAdd(field.Id, property);
                        }
                    }
                }

                foreach (var value in Values.Value[id].EnumerateObject())
                {
                    var colon = value.Name.IndexOf(':');
                    var field = colon < 0 ? value.Name : value.Name.Substring(0, colon);
                    var level = colon < 0 ? 0 : int.Parse(value.Name.Substring(colon + 1));
                    var text = value.Value.GetString()!;
                    var property = properties[field];
                    var read = property.GetValue(made);
                    var where = $"{type.FullName} ({id}) {value.Name}";
                    var generic = property.PropertyType.IsGenericType ? property.PropertyType.GetGenericTypeDefinition() : null;
                    if (generic == typeof(ObjectList<>))
                    {
                        // The property reads level 1's list of a field with levels.
                        if (level <= 1)
                        {
                            var entries = (IReadOnlyList<object>)property.PropertyType.GetProperty("Entries", BindingFlags.Instance | BindingFlags.NonPublic)!.GetValue(read)!;
                            Assert.True(text.Split(',').Distinct().SequenceEqual(entries.Select(Id)), where);
                        }
                    }
                    else if (generic == typeof(ObjectRef<>))
                    {
                        if (level <= 1)
                        {
                            var entry = property.PropertyType.GetProperty("Entry", BindingFlags.Instance | BindingFlags.NonPublic)!.GetValue(read)!;
                            Assert.True(text == Id(entry), where);
                        }
                    }
                    else if (generic == typeof(Levels<>))
                    {
                        var element = property.PropertyType.GetGenericArguments()[0];
                        var expected = parse.MakeGenericMethod(element).Invoke(null, new object?[] { text });
                        var one = property.PropertyType.GetProperty("Item")!.GetValue(read, new object[] { level });
                        Assert.True(Equals(expected, one), $"{where}: {one}, not {expected}");
                    }
                    else
                    {
                        var expected = parse.MakeGenericMethod(property.PropertyType).Invoke(null, new object?[] { text });
                        Assert.True(Equals(expected, read), $"{where}: {read}, not {expected}");
                    }

                    checkedValues++;
                }
            }

            Assert.True(checkedValues > 100000, checkedValues + " values");
        }

        // The id an entry of a list or a reference stands for: a class's standard id, or the id text of one with no class.
        private static string Id(object entry)
        {
            return entry is Type type ? type.GetCustomAttribute<StandardAttribute>(false)!.Id : (string)entry;
        }
    }

    /// <summary>The tests that set <see cref="StandardValues.Read"/>, a static, run one after another.</summary>
    [CollectionDefinition(Name)]
    public class StandardValuesCollection
    {
        public const string Name = "StandardValues";
    }
}
