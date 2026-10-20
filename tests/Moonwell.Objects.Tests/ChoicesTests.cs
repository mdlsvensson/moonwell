using System.Reflection;

namespace Moonwell.Objects.Tests
{
    /// <summary>A modder's unit: inside the class, AttackType.Magic is the enum, the property of that name holding it.</summary>
    public class Mage : Moonwell.Objects.Human.Units.Footman
    {
        public Mage()
        {
            AttackType = AttackType.Magic;
            Attack1TargetsAllowed = Targets.Air | Targets.Enemies;
            DeathType = DeathType.CanRaiseDoesNotDecay;
        }
    }

    /// <summary>A modder's ability: the targets' property is not named like their enum, which it holds for each level.</summary>
    public class Bolt : Moonwell.Objects.Human.Abilities.StormBolt
    {
        public Bolt()
        {
            TargetsAllowed = Levels(Targets.Air | Targets.Enemies, Targets.Ground);
        }
    }

    public class ChoicesTests
    {
        private static readonly Assembly Objects = typeof(StandardAttribute).Assembly;

        private static IEnumerable<Type> Enums()
        {
            return Objects.GetTypes().Where(type => type.IsEnum && type.Namespace == "Moonwell.Objects" && type.IsPublic);
        }

        [Fact]
        public void a_choice_field_is_its_enum()
        {
            Assert.Equal(typeof(Moonwell.AttackType), typeof(StandardUnit).GetProperty("AttackType")!.PropertyType);
            Assert.Equal(typeof(Moonwell.DefenseType), typeof(StandardUnit).GetProperty("DefenseType")!.PropertyType);
            Assert.Equal(typeof(Levels<DefenseTypeInt>), typeof(NightElf.Abilities.Root).GetProperty("UprootedDefenseType")!.PropertyType);
            Assert.Equal(typeof(Targets), typeof(StandardUnit).GetProperty("Attack1TargetsAllowed")!.PropertyType);
            Assert.Equal(typeof(DeathType), typeof(StandardUnit).GetProperty("DeathType")!.PropertyType);
            Assert.Equal(typeof(Levels<Targets>), typeof(StandardAbility).GetProperty("TargetsAllowed")!.PropertyType);
            Assert.Equal(typeof(MoveType), typeof(StandardUnit).GetProperty("Type")!.PropertyType);
            Assert.Equal(typeof(ArmorType), typeof(StandardUnit).GetProperty("ArmorType")!.PropertyType);
        }

        [Fact]
        public void attack_and_defense_types_are_core_moonwells_not_the_libraries()
        {
            // The damage package names them too, and it cannot reference Moonwell.Objects (C19).
            Assert.Null(Objects.GetType("Moonwell.Objects.AttackType"));
            Assert.Null(Objects.GetType("Moonwell.Objects.DefenseType"));
            Assert.NotNull(Objects.GetType("Moonwell.Objects.DefenseTypeInt"));
            Assert.Equal(
                new[] { "Normal", "Small", "Medium", "Large", "Fort", "Hero", "Divine", "None" },
                Enum.GetNames<Moonwell.DefenseType>());
            Assert.Equal(6, (int)Moonwell.DefenseType.Divine);
        }

        [Fact]
        public void a_core_enums_game_values_have_its_members()
        {
            var table = Objects.GetType("Moonwell.Objects.CoreChoices", true)!;
            foreach (var core in new[] { typeof(Moonwell.AttackType), typeof(Moonwell.DefenseType) })
            {
                var texts = table.GetNestedType(core.Name, BindingFlags.NonPublic)!;
                Assert.Equal(
                    core.GetFields(BindingFlags.Public | BindingFlags.Static).Select(field => (field.Name, Convert.ToInt32(field.GetValue(null)))),
                    texts.GetFields(BindingFlags.Public | BindingFlags.Static).Select(field => (field.Name, Convert.ToInt32(field.GetValue(null)))));
                Assert.All(texts.GetFields(BindingFlags.Public | BindingFlags.Static), field => Assert.NotNull(field.GetCustomAttribute<GameValueAttribute>()));
            }
        }

        [Fact]
        public void attack_type_has_the_editors_values_in_order()
        {
            Assert.Equal(
                new[] { "Unknown", "Normal", "Pierce", "Siege", "Spells", "Chaos", "Magic", "Hero" },
                Enum.GetNames<AttackType>());
            Assert.Equal(6, (int)AttackType.Magic);
        }

        [Fact]
        public void a_declared_choice_is_put_as_the_enum()
        {
            var mage = new Mage();

            Assert.Equal(AttackType.Magic, mage.AttackType);
            Assert.Equal(Targets.Air | Targets.Enemies, mage.Attack1TargetsAllowed);
            Assert.Equal(AttackType.Magic, ((StandardUnit)mage).Declared["ua1t"]);
            Assert.Equal(Targets.Ground, new Bolt().TargetsAllowed[2]);
        }

        [Fact]
        public void set_by_id_takes_the_enum_and_names_it_otherwise()
        {
            var paladin = new OpenPaladin();

            paladin.Put("ua1t", AttackType.Chaos);

            Assert.Equal(AttackType.Chaos, paladin.AttackType);
            var error = Assert.Throws<ArgumentException>(() => paladin.Put("ua1t", "chaos"));
            Assert.StartsWith("StandardHero field ua1t holds AttackType, not string", error.Message);
        }

        [Theory]
        [InlineData("magic", AttackType.Magic)]
        [InlineData("MAGIC", AttackType.Magic)]
        [InlineData(" hero ", AttackType.Hero)]
        [InlineData("3", AttackType.Siege)]
        [InlineData("nonsense", AttackType.Unknown)]
        [InlineData(null, AttackType.Unknown)]
        public void a_choice_reads_its_text(string? text, AttackType expected)
        {
            Assert.Equal(expected, StandardValues.Parse<AttackType>(text));
        }

        [Theory]
        [InlineData("divine", Moonwell.DefenseType.Divine)]
        [InlineData("Fort", Moonwell.DefenseType.Fort)]
        [InlineData("none", Moonwell.DefenseType.None)]
        public void a_core_choice_reads_its_text(string text, Moonwell.DefenseType expected)
        {
            Assert.Equal(expected, StandardValues.Parse<Moonwell.DefenseType>(text));
        }

        [Theory]
        [InlineData("air,enemies", Targets.Air | Targets.Enemies)]
        [InlineData("aliv", Targets.Alive)]
        [InlineData("alive", Targets.Alive)]
        [InlineData("ground,enemy,vuln", Targets.Ground | Targets.Enemies | Targets.Vulnerable)]
        [InlineData("air, alli ,ally", Targets.Air | Targets.Allies)]
        [InlineData("0", (Targets)0)]
        [InlineData("", (Targets)0)]
        [InlineData("air,nothing", Targets.Air)]
        public void a_list_reads_its_texts_and_their_other_spellings(string text, Targets expected)
        {
            Assert.Equal(expected, StandardValues.Parse<Targets>(text));
        }

        [Fact]
        public void a_value_stored_as_a_number_reads_its_number()
        {
            Assert.Equal(DeathType.CanRaiseDoesDecay, StandardValues.Parse<DeathType>("3"));
            Assert.Equal(DefenseTypeInt.Normal, StandardValues.Parse<DefenseTypeInt>("4"));
            Assert.Equal(TeamColor.MatchOwningPlayer, StandardValues.Parse<TeamColor>("-1"));
            Assert.Equal(MorphFlags.Uninterruptable | MorphFlags.ImmediateTakeOff, StandardValues.Parse<MorphFlags>("5"));
            Assert.Equal((InteractionFlags)16, StandardValues.Parse<InteractionFlags>("16"));
        }

        [Fact]
        public void a_word_the_standard_data_spells_in_another_case_reads()
        {
            Assert.Equal(UnitClass.Mechanical | UnitClass.Undead, StandardValues.Parse<UnitClass>("undead,Mechanical"));
            Assert.Equal(OcclusionType.NoOcclusion, StandardValues.Parse<OcclusionType>("No Occlusion"));
            Assert.Equal(ShadowImage.Shadow, StandardValues.Parse<ShadowImage>("shadow"));
        }

        [Fact]
        public void every_member_has_its_game_value_and_a_flag_is_one_bit_the_games_integers_hold()
        {
            var enums = Enums().ToList();
            Assert.Equal(39, enums.Count);
            foreach (var type in enums)
            {
                var flags = type.IsDefined(typeof(FlagsAttribute), false);
                foreach (var field in type.GetFields(BindingFlags.Public | BindingFlags.Static))
                {
                    var value = field.GetCustomAttribute<GameValueAttribute>();
                    Assert.True(value != null, $"{type.Name}.{field.Name} has no GameValue");
                    Assert.NotEqual("", value!.Text);
                    if (flags)
                    {
                        var number = Convert.ToInt64(field.GetValue(null));
                        Assert.True(number > 0 && number <= int.MaxValue && (number & (number - 1)) == 0, $"{type.Name}.{field.Name} is {number}, not one of bits 0 to 30");
                    }
                }
            }
        }

        [Fact]
        public void the_enums_avoid_the_names_of_namespace_moonwell()
        {
            var runtime = typeof(Moonwell.Unit).Assembly.GetExportedTypes().Where(type => type.Namespace == "Moonwell").Select(type => type.Name).ToHashSet();

            foreach (var type in Enums())
            {
                Assert.DoesNotContain(type.Name, runtime);
            }
        }
    }
}
