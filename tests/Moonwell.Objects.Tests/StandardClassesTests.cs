using System.Reflection;
using System.Text.Json;

namespace Moonwell.Objects.Tests
{
    /// <summary>A Paladin of a map's own that sets nothing.</summary>
    public class Squire : Moonwell.Objects.Human.Units.Paladin
    {
    }

    /// <summary>A Footman of a map's own that sets its most life.</summary>
    public class Sentinel : Moonwell.Objects.Human.Units.Footman
    {
        public Sentinel()
        {
            MaxLife = 900;
        }
    }

    /// <summary>A Footman of a map's own a tenth larger than the footman.</summary>
    public class Grown : Moonwell.Objects.Human.Units.Footman
    {
        public Grown()
        {
            Scale = Scale * 1.1f;
        }
    }

    /// <summary>A Paladin of a map's own that sets its life, then its strength.</summary>
    public class LifeThenStrength : Moonwell.Objects.Human.Units.Paladin
    {
        public LifeThenStrength()
        {
            MaxLife = 800;
            Strength = 30;
        }
    }

    /// <summary>A Paladin of a map's own that sets its strength, then its life.</summary>
    public class StrengthThenLife : Moonwell.Objects.Human.Units.Paladin
    {
        public StrengthThenLife()
        {
            Strength = 30;
            MaxLife = 800;
        }
    }

    /// <summary>A Paladin of a map's own that adds to the life it starts with.</summary>
    public class Stalwart : Moonwell.Objects.Human.Units.Paladin
    {
        public Stalwart()
        {
            MaxLife += 200;
        }
    }

    /// <summary>A Paladin of a map's own that sets its strength and intelligence alone.</summary>
    public class Scholarly : Moonwell.Objects.Human.Units.Paladin
    {
        public Scholarly()
        {
            Strength = 30;
            Intelligence = 20;
        }
    }

    /// <summary>A Paladin of a map's own that lowers its most mana below what it starts with.</summary>
    public class Drained : Moonwell.Objects.Human.Units.Paladin
    {
        public Drained()
        {
            MaxMana = 100;
        }
    }

    /// <summary>A Paladin of a map's own that starts with more mana than its most.</summary>
    public class Overfull : Moonwell.Objects.Human.Units.Paladin
    {
        public Overfull()
        {
            Mana = 1000;
            MaxMana = 300;
        }
    }

    /// <summary>A Paladin of a map's own that reads its own default object in its constructor, which MW0012 rejects.</summary>
    public class ReadsOwnDefault : Moonwell.Objects.Human.Units.Paladin
    {
        public ReadsOwnDefault()
        {
            Tooltip = "Has " + Default.MaxLife;
        }
    }

    /// <summary>A Paladin of a map's own that reads its class's default object in its constructor, which MW0012 rejects.</summary>
    public class ReadsOwnClassDefault : Moonwell.Objects.Human.Units.Paladin
    {
        public ReadsOwnClassDefault()
        {
            Tooltip = "Has " + Moonwell.Unit.Default<ReadsOwnClassDefault>().MaxLife;
        }
    }

    /// <summary>A Footman of a map's own whose default is first asked for from an object of it.</summary>
    public class Lookout : Moonwell.Objects.Human.Units.Footman
    {
        public Lookout()
        {
            MaxLife = 300;
        }
    }

    /// <summary>A Paladin of a map's own that sets its level, which a hero does not declare.</summary>
    public class Veteran : Moonwell.Objects.Human.Units.Paladin
    {
        public Veteran()
        {
            Level = 3;
        }
    }

    /// <summary>A Footman of a map's own that sets its shared fields by their live names.</summary>
    public class Commander : Moonwell.Objects.Human.Units.Footman
    {
        public Commander()
        {
            Name = "Commander";
            MaxMana = 300.9f;
            Mana = 100;
            Level = 4;
            Scale = 1.25f;
            FlyHeight = 50f;
            Tooltip = Name + " leads";
        }
    }

    /// <summary>A Potion of Healing of a map's own with more charges.</summary>
    public class BigPotion : Moonwell.Objects.Items.PotionOfHealing
    {
        public BigPotion()
        {
            Charges = 3;
        }
    }

    /// <summary>A Phoenix of a map's own: the Phoenix's standard regeneration, -25, is below World Editor's range of 0 to 1000.</summary>
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

    /// <summary>A Phoenix of a map's own that puts the Phoenix's own regeneration past World Editor's range on purpose.</summary>
    [BeyondLimits(nameof(HitPointRegen))]
    public class WarmFirebird : Moonwell.Objects.Human.Units.Phoenix
    {
        public WarmFirebird()
        {
            HitPointRegen = -25f;
        }
    }

    /// <summary>A standard class that cannot be made by itself: no class of Moonwell.Objects is one.</summary>
    [Standard("Hmkg")]
    public abstract class Unmade : TestUnitBase
    {
    }

    /// <summary>A class of a map's own on a standard class that cannot be made by itself.</summary>
    public class FromUnmade : Unmade
    {
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
            StandardValues.HeroBonus = null;
        }

        public void Dispose()
        {
            StandardValues.Read = null;
            StandardValues.HeroBonus = null;
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

            // With no build, a hero's bonus of each attribute is 0: its life is its base.
            Assert.Equal(int.Parse(Base("Hpal", "uhpm")), paladin.MaxLife);
            Assert.Equal(100, paladin.MaxLife);
            Assert.Equal("Paladin", paladin.Name);
            Assert.Equal(AttackType.Hero, paladin.AttackType);
            Assert.Equal(
                new object[] { typeof(Moonwell.Objects.Human.Abilities.HolyLight), typeof(Moonwell.Objects.Human.Abilities.DivineShield), typeof(Moonwell.Objects.Human.Abilities.Resurrection), typeof(Moonwell.Objects.Human.Abilities.DevotionAura) },
                paladin.HeroAbilities.Entries);
        }

        [Fact]
        public void a_standard_units_scale_is_the_games()
        {
            // The game scales them so in Definitive Edition graphics (game-facts, seen on 3.0.1); a scale of their own is read from it.
            Assert.Equal(0.8f, new Moonwell.Objects.Human.Units.Footman().Scale);
            Assert.Equal(0.935f, new Moonwell.Objects.Human.Units.Paladin().Scale);
            Assert.Equal(0.85f, new Moonwell.Objects.Human.Units.Peasant().Scale);
            Assert.Equal(0.8f * 1.1f, new Grown().Scale);
        }

        [Fact]
        public void a_standard_units_scale_with_no_definitive_edition_one_is_the_reforged_one()
        {
            // The game reads :hd when a unit has no :de value (game-facts, seen on 3.0.1): an Assassin's 1.2 (sd 1.3), a Guardian Golem's 1.0 (sd 1.5).
            Assert.Equal(1.2f, new Moonwell.Objects.Creeps.Units.Assassin().Scale);
            Assert.Equal(1.0f, new Moonwell.Objects.Creeps.Units.GuardianGolem().Scale);
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

            Assert.Equal(900, sentinel.MaxLife);
            Assert.Equal(new[] { "uhpm" }, DeclaredOf(sentinel).Keys);
            Assert.Equal(900, DeclaredOf(sentinel)["uhpm"]);
        }

        [Fact]
        public void a_shared_field_set_by_its_live_name_is_declared_and_read_back_in_its_fields_type()
        {
            var commander = new Commander();

            Assert.Equal("Commander", commander.Name);
            Assert.Equal(300f, commander.MaxMana);
            Assert.Equal(100f, commander.Mana);
            Assert.Equal(4, commander.Level);
            Assert.Equal(1.25f, commander.Scale);
            Assert.Equal(50f, commander.FlyHeight);
            Assert.Equal("Commander leads", commander.Tooltip);
            Assert.Equal(new[] { "ulev", "umpi", "umpm", "umvh", "unam", "usca", "utip" }, DeclaredOf(commander).Keys.OrderBy(k => k, StringComparer.Ordinal));
            // A float put into an int field is truncated, as the live setter does.
            Assert.Equal(300, DeclaredOf(commander)["umpm"]);
            Assert.Equal(3, new BigPotion().Charges);
            Assert.Equal(3, DeclaredOf(new BigPotion())["iuse"]);
        }

        [Fact]
        public void a_heros_life_reads_the_total()
        {
            StandardValues.HeroBonus = () => (25f, 15f);

            Assert.Equal(100 + (22 * 25), new Moonwell.Objects.Human.Units.Paladin().MaxLife);
            Assert.Equal(100 + (30 * 25), new Scholarly().MaxLife);
            Assert.Equal(0 + (20 * 15), new Scholarly().MaxMana);
            Assert.Empty(new Scholarly().Totals);
        }

        [Fact]
        public void a_heros_life_set_is_the_total_whatever_the_order()
        {
            StandardValues.HeroBonus = () => (25f, 15f);

            foreach (var hero in new Moonwell.Objects.StandardHero[] { new LifeThenStrength(), new StrengthThenLife() })
            {
                Assert.Equal(800, hero.Totals["uhpm"]);
                Assert.Equal(800, hero.MaxLife);
                Assert.Equal(30, hero.Strength);
                Assert.False(hero.Declared.ContainsKey("uhpm"));
            }
        }

        [Fact]
        public void life_plus_200_on_a_hero_adds_200_to_the_total()
        {
            StandardValues.HeroBonus = () => (25f, 15f);

            var stalwart = new Stalwart();

            Assert.Equal(100 + (22 * 25) + 200, stalwart.Totals["uhpm"]);
            Assert.Equal(850, stalwart.MaxLife);
            Assert.Empty(stalwart.Declared);
        }

        [Fact]
        public void a_heros_mana_starts_with_its_intelligence_up_to_its_most()
        {
            StandardValues.HeroBonus = () => (25f, 15f);

            // The Paladin's base mana is 0 and its starting mana 100: 17 intelligence add 255 to each, and it starts full.
            var paladin = new Moonwell.Objects.Human.Units.Paladin();
            Assert.Equal(255, paladin.MaxMana);
            Assert.Equal(255, paladin.Mana);
            Assert.Equal(100, new Drained().Mana);
            Assert.Equal(100, new Drained().Totals["umpm"]);
        }

        [Fact]
        public void a_heros_mana_put_above_its_most_reads_its_most()
        {
            StandardValues.HeroBonus = () => (25f, 15f);

            Assert.Equal(300, new Overfull().Mana);
            Assert.Equal(300, new Overfull().MaxMana);
        }

        [Fact]
        public void a_classs_own_default_object_read_in_its_constructor_throws_with_what_to_read()
        {
            var error = Assert.Throws<MoonwellException>(() => new ReadsOwnDefault());
            var byClass = Assert.Throws<MoonwellException>(() => Moonwell.Unit.Default<ReadsOwnClassDefault>());

            Assert.Equal("ReadsOwnDefault's default object is read while its constructor runs; read the field itself", error.Message);
            Assert.Equal("ReadsOwnClassDefault's default object is read while its constructor runs; read the field itself", byClass.Message);
        }

        [Fact]
        public void a_classs_default_object_asked_first_from_an_object_is_the_classs()
        {
            var first = new Lookout().Default;

            Assert.IsType<Lookout>(first);
            Assert.Equal(300f, first.MaxLife);
            Assert.Same(first, Moonwell.Unit.Default<Lookout>());
        }

        [Fact]
        public void a_heros_level_in_its_constructor_throws()
        {
            var error = Assert.Throws<InvalidOperationException>(() => new Veteran());

            Assert.Equal("a hero starts at level 1; its level is not declared", error.Message);
            // Its standard class sets its unit level by id, and the hero reads level 1.
            Assert.Equal(1, new Moonwell.Objects.Human.Units.Paladin().Level);
        }

        [Fact]
        public void a_declared_class_reaches_every_field_of_its_default_object()
        {
            var squire = Moonwell.Unit.Default<Squire>();

            Assert.Same(squire, squire.Default);
            Assert.Equal("Paladin", squire.Default.Name);
            Assert.Equal(squire.GoldCost, squire.Default.GoldCost);
            Assert.True(squire.Default.IsGone);
        }

        [Fact]
        public void a_map_classs_own_value_is_checked()
        {
            var error = Assert.Throws<FieldRuleException>(() => new Giant());

            Assert.Equal("Giant.MaxLife is 600000; Hit Points Maximum (Base) (uhpm) allows 1 to 500000; [BeyondLimits] goes past it", error.Message);
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

            Assert.Equal("ColdFirebird.HitPointRegen is -25; Hit Points Regeneration Rate (uhpr) allows 0 to 1000; [BeyondLimits] goes past it", error.Message);
        }

        [Fact]
        public void the_same_value_put_past_the_limits_on_purpose_builds()
        {
            // The game's own Phoenix holds -25, so a negative there is part of the range [BeyondLimits] lifts (ruling R14).
            Assert.Equal(-25f, new WarmFirebird().HitPointRegen);
        }

        [Fact]
        public void a_standard_class_that_cannot_be_made_by_itself_is_an_error_that_names_it()
        {
            var error = Assert.Throws<InvalidOperationException>(() => new FromUnmade().MaxLife);

            Assert.StartsWith("Moonwell.Objects.Tests.Unmade, a standard class, could not be made by itself to count the values its constructor sets: ", error.Message);
        }

        /// <summary>
        /// Guards every rule [BeyondLimits] never lifts (no negative, a name, a choice): the game's own data must fit them, or a
        /// map could not write a standard value even on purpose. Each value of each standard class, under the base tables and
        /// every balance set, is checked by the build's own checks as a map class on it that lifts the property would put it.
        /// </summary>
        [Fact]
        public void every_standard_value_put_by_a_map_class_that_lifts_every_limit_is_allowed()
        {
            var parse = typeof(StandardValues).GetMethod("Parse", BindingFlags.Static | BindingFlags.NonPublic)!;
            var rulesType = typeof(RuleChecks).GetNestedType("Rules", BindingFlags.NonPublic)!;
            var check = typeof(RuleChecks).GetMethod("Check", BindingFlags.Static | BindingFlags.NonPublic)!;
            var rulesOf = new Dictionary<object, object>();
            var broken = new List<string>();
            var checkedValues = 0;
            foreach (var type in Objects.GetTypes().Where(type => type.GetCustomAttribute<StandardAttribute>(false) != null))
            {
                var id = type.GetCustomAttribute<StandardAttribute>(false)!.Id;
                var properties = new Dictionary<string, (PropertyInfo Property, FieldAttribute Field)>(StringComparer.Ordinal);
                for (var chain = type; chain != null; chain = chain.BaseType)
                {
                    foreach (var property in chain.GetProperties(BindingFlags.Instance | BindingFlags.Public | BindingFlags.DeclaredOnly))
                    {
                        if (property.GetCustomAttribute<FieldAttribute>(false) is { } field)
                        {
                            properties.TryAdd(field.Id, (property, field));
                        }
                    }
                }

                var shared = SharedFieldsOf(type);
                foreach (var (key, text) in AllValues.Value[id])
                {
                    var colon = key.IndexOf(':');
                    var fieldId = colon < 0 ? key : key.Substring(0, colon);
                    int? level = colon < 0 ? null : int.Parse(key.Substring(colon + 1));
                    object? rules;
                    Type element;
                    if (shared.TryGetValue(fieldId, out var one))
                    {
                        // A shared field's rules are on its kind base's [SharedField], as a class that lifts its member has them.
                        if (!rulesOf.TryGetValue(one.Shared, out rules))
                        {
                            rulesOf[one.Shared] = rules = Activator.CreateInstance(rulesType, "Lifted", one.Shared, true)!;
                        }

                        element = one.Held;
                    }
                    else
                    {
                        var (property, field) = properties[fieldId];
                        var generic = property.PropertyType.IsGenericType ? property.PropertyType.GetGenericTypeDefinition() : null;
                        if (generic == typeof(ObjectList<>) || generic == typeof(ObjectRef<>))
                        {
                            // A list's only rule is its count, which [BeyondLimits] lifts.
                            continue;
                        }

                        if (!rulesOf.TryGetValue(property, out rules))
                        {
                            // The rules as a class that lifts the property has them (lifted: true).
                            rulesOf[property] = rules = Activator.CreateInstance(rulesType, "Lifted", property, field, true)!;
                        }

                        element = generic == typeof(Levels<>) ? property.PropertyType.GetGenericArguments()[0] : property.PropertyType;
                    }

                    var value = parse.MakeGenericMethod(element).Invoke(null, new object?[] { text });
                    try
                    {
                        check.Invoke(null, new[] { rules, level, value, null });
                    }
                    catch (TargetInvocationException error) when (error.InnerException is FieldRuleException rule)
                    {
                        broken.Add($"{id} {key} \"{text}\": {rule.Message}");
                    }

                    checkedValues++;
                }
            }

            Assert.True(broken.Count == 0, broken.Count + " standard values break a rule [BeyondLimits] does not lift:\n" + string.Join("\n", broken.Take(40)));
            Assert.True(checkedValues > 100000, checkedValues + " values");
        }

        // The declared values of an object of a generated class, its kind base's protected Fields.
        private static Declared FieldsOf(object made)
        {
            for (var type = made.GetType(); type != null; type = type.BaseType)
            {
                var property = type.GetProperty("Fields", BindingFlags.Instance | BindingFlags.NonPublic | BindingFlags.DeclaredOnly);
                if (property != null)
                {
                    return (Declared)property.GetValue(made)!;
                }
            }

            throw new InvalidOperationException(made.GetType() + " has no Fields");
        }

        // The shared fields of a standard class by id:the [SharedField] of its kind base and the type its table says the field holds.
        private static Dictionary<string, (SharedFieldAttribute Shared, Type Held)> SharedFieldsOf(Type type)
        {
            var held = (IReadOnlyDictionary<string, Type>?)null;
            for (var chain = type; chain != null && held == null; chain = chain.BaseType)
            {
                held = (IReadOnlyDictionary<string, Type>?)chain.GetField("fieldTypes", BindingFlags.Static | BindingFlags.NonPublic | BindingFlags.DeclaredOnly)?.GetValue(null);
            }

            var shared = new Dictionary<string, (SharedFieldAttribute, Type)>(StringComparer.Ordinal);
            foreach (var attribute in type.GetCustomAttributes<SharedFieldAttribute>(true))
            {
                shared[attribute.Id] = (attribute, held![attribute.Id]);
            }

            return shared;
        }

        // Every value of every standard object, under the base tables and each balance set (a set's empty text is no value), by id.
        private static readonly Lazy<Dictionary<string, List<(string Key, string Text)>>> AllValues = new Lazy<Dictionary<string, List<(string Key, string Text)>>>(ReadAllValues);

        private static Dictionary<string, List<(string Key, string Text)>> ReadAllValues()
        {
            var all = new Dictionary<string, List<(string Key, string Text)>>(StringComparer.Ordinal);
            foreach (var kind in new[] { "unit", "item", "destructable", "doodad", "ability", "buff", "upgrade" })
            {
                using var stream = Objects.GetManifestResourceStream($"Moonwell.Objects.Values.{kind}.json")!;
                using var document = JsonDocument.Parse(stream);
                foreach (var entry in document.RootElement.GetProperty("objects").EnumerateObject())
                {
                    var values = new List<(string Key, string Text)>();
                    var sets = entry.Value.TryGetProperty("sets", out var found) ? found.EnumerateObject().Select(set => set.Value) : Enumerable.Empty<JsonElement>();
                    foreach (var group in new[] { entry.Value.GetProperty("values") }.Concat(sets))
                    {
                        foreach (var value in group.EnumerateObject())
                        {
                            var text = value.Value.GetString()!;
                            if (text.Length > 0)
                            {
                                values.Add((value.Name, text));
                            }
                        }
                    }

                    all[entry.Name] = values;
                }
            }

            return all;
        }

        [Fact]
        public void with_values_from_a_build_an_unset_field_reads_the_data_sets_value()
        {
            // A map on melee_v0: the Paladin's speed there is 270, the base tables' (which its constructor sets) 300.
            StandardValues.Read = (id, field, level) => id == "Hpal" && field == "umvs" ? "270" : id == "hfoo" && field == "uabi" ? "Adef" : null;

            Assert.Equal(270, new Squire().Speed);
            Assert.Equal(270, new Moonwell.Objects.Human.Units.Paladin().Speed);
            // The set has no value for the Paladin's hit points: the base tables' value is not used either.
            Assert.Equal(0, new Squire().MaxLife);
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

            Assert.True(new Talon().DefaultActiveAbility.Is<Moonwell.Objects.NightElf.Abilities.FaerieFire>());
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

                var shared = SharedFieldsOf(type);
                foreach (var value in Values.Value[id].EnumerateObject())
                {
                    var colon = value.Name.IndexOf(':');
                    var field = colon < 0 ? value.Name : value.Name.Substring(0, colon);
                    var level = colon < 0 ? 0 : int.Parse(value.Name.Substring(colon + 1));
                    var text = value.Value.GetString()!;
                    var where = $"{type.FullName} ({id}) {value.Name}";
                    if (shared.TryGetValue(field, out var sharedField))
                    {
                        // A shared field has no property: its base value is in the declared values (a hero's live member reads a total).
                        var stored = typeof(Declared).GetMethod("Get")!.MakeGenericMethod(sharedField.Held).Invoke(FieldsOf(made), new[] { made, field });
                        var expectedShared = parse.MakeGenericMethod(sharedField.Held).Invoke(null, new object?[] { text });
                        Assert.True(Equals(expectedShared, stored), $"{where}: {stored}, not {expectedShared}");
                        checkedValues++;
                        continue;
                    }

                    var property = properties[field];
                    var read = property.GetValue(made);
                    var generic = property.PropertyType.IsGenericType ? property.PropertyType.GetGenericTypeDefinition() : null;
                    if (generic == typeof(ObjectList<>))
                    {
                        // The property reads level 1's list of a field with levels, each entry trimmed ("Aspo, Aspt").
                        if (level <= 1)
                        {
                            var entries = (IReadOnlyList<object>)property.PropertyType.GetProperty("Entries", BindingFlags.Instance | BindingFlags.NonPublic)!.GetValue(read)!;
                            var ids = text.Split(',').Select(id => id.Trim()).Where(id => id.Length > 0).Distinct();
                            Assert.True(ids.SequenceEqual(entries.Select(Id)), where);
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
                        // The constructor sets an ability's levels up to its count; the values past it are the build's to read.
                        if (Values.Value[id].TryGetProperty("alev", out var count) && level > int.Parse(count.GetString()!))
                        {
                            continue;
                        }

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

    /// <summary>The tests that set <see cref="StandardValues.Read"/> or <see cref="StandardValues.HeroBonus"/>, statics, or read a hero's totals, run one after another.</summary>
    [CollectionDefinition(Name)]
    public class StandardValuesCollection
    {
        public const string Name = "StandardValues";
    }
}
