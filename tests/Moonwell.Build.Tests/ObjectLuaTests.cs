using System.Globalization;
using System.Reflection;
using System.Reflection.Emit;
using System.Text;
using System.Text.RegularExpressions;
using War3Net.Build.Extensions;
using War3Net.Build.Object;
using War3Net.Build.Script;
using Xunit;

namespace Moonwell.Build.Tests
{
    [Collection("Sample")]
    public sealed class ObjectLuaTests : IClassFixture<ObjectLuaTests.CompiledObjects>
    {
        private readonly CompiledObjects compiled;

        public ObjectLuaTests(CompiledObjects compiled)
        {
            this.compiled = compiled;
        }

        [Fact]
        public void declared_value_reads_in_lua()
        {
            var paladin = int.Parse(compiled.Standard.Value("Hpal", "ugol", 0, null)!, CultureInfo.InvariantCulture);

            var run = compiled.Run();

            Assert.Empty(run.Errors);
            // Damage is a float, so Lua prints it with its point (C18); so is a unit's MaxLife, Arthas's doubled total.
            Assert.Contains($"objects: {2 * paladin} 200.0 1300.0", run.Lines);
        }

        [Fact]
        public void declared_shared_value_reads_in_lua()
        {
            using var lua = compiled.Start();

            // The Paladin's total life is 100 + 22 * 25, which Arthas doubles; his mana starts full at 0 + 17 * 15. The values are
            // the totals the build wrote, read through the kind base's hooks with no handle.
            Assert.Equal("1300.0 255.0 1300.0 Arthas", Query(lua, "return Objects.Report.Shared()"));
            Assert.Equal("1300.0 Arthas", Query(lua, "local a = Objects.Arthas(); return tostring(a:getMaxLife()) .. ' ' .. a:getName()"));
            Assert.Equal("1300.0 Dark Arthas", Query(lua, "local a = Objects.DarkArthas(); return tostring(a:getMaxLife()) .. ' ' .. a:getName()"));
            Assert.Equal("600.0", Query(lua, "return tostring(Objects.StrongFootman():getMaxLife())"));
            Assert.Contains("[\"uhpm\"] = 1300", Regex.Match(CompiledObjects.ValuesOf(compiled.Script), "\\[\"Objects\\.Arthas\"\\] = \\{[^\\n]*").Value);
        }

        [Fact]
        public void the_default_object_is_the_same_object_from_a_unit_and_from_its_class()
        {
            using var lua = compiled.Start();

            Assert.True(Truth(lua, "return Objects.Report.SameDefault()"));
            Assert.True(Truth(lua, "return System.is(Moonwell.Widget.Default(Objects.Arthas), Objects.Arthas)"));
        }

        [Fact]
        public void shared_members_in_a_constructor_call_no_native_in_lua()
        {
            using var lua = compiled.Start();

            // Every global the game would give is logged from here: a native called with no handle would show.
            lua.DoString("natives = {}; setmetatable(_G, { __index = function(_, name) return function() natives[#natives + 1] = name; return nil end end })");
            lua.DoString("for _, id in ipairs({ 'H001', 'H002', 'hfoo' }) do Moonwell.ObjectTypes.UnitMaker(Moonwell.ObjectTypes.FourCC(id))() end");

            Assert.Equal(string.Empty, Query(lua, "return table.concat(natives, ',')"));
        }

        [Fact]
        public void type_values_are_written_when_the_map_reads_default()
        {
            // Report reads Default, MaxLife, Mana and Name: every unit and hero type has those fields, a hero its totals, and a map's class its own values.
            var values = compiled.Script.Substring(compiled.Script.LastIndexOf("MoonwellTypeValues = {", StringComparison.Ordinal));

            Assert.Contains($"[{ObjectLua.FourCC("H001")}] = {{ [\"uhpm\"] = 1300, [\"umpi\"] = 255, [\"unam\"] = \"Arthas\" }}", values);
            Assert.Contains($"[{ObjectLua.FourCC("hfoo")}] = {{ [\"uhpm\"] = 600, [\"umpi\"] = 0, [\"unam\"] = \"Footman\" }}", values);
            Assert.Contains($"[{ObjectLua.FourCC("Hamg")}] = {{ [\"uhpm\"] = 450, [\"umpi\"] = 285, [\"unam\"] = \"Archmage\" }}", values);
            // No item field is read.
            Assert.DoesNotContain($"[{ObjectLua.FourCC("ratf")}] =", values);
            Assert.DoesNotContain("umpm", values);
        }

        [Fact]
        public void a_plain_footmans_default_reads_its_standard_value()
        {
            using var lua = compiled.Start();

            // The Objects map's StrongFootman replaces the footman; the knight and the archmage have no class of the map's. A unit
            // the game made has its type id and a plain class; its default object has neither handle nor class of the map's.
            Assert.Equal("885.0 Knight", Query(lua, "local u = Moonwell.Unit(); u.TypeId = Moonwell.ObjectTypes.FourCC('hkni'); return Objects.Report.DefaultOf(u)"));
            Assert.Equal("450.0 Archmage", Query(lua, "local h = Moonwell.Hero(); h.TypeId = Moonwell.ObjectTypes.FourCC('Hamg'); return Objects.Report.DefaultOf(h)"));
            Assert.Equal("600.0 Footman", Query(lua, "local u = Moonwell.Unit(); u.TypeId = Moonwell.ObjectTypes.FourCC('hfoo'); return Objects.Report.DefaultOf(u)"));
        }

        [Fact]
        public void type_values_read_the_types_the_map_makes_or_changes_in_world_editor()
        {
            // World Editor changed the knight and made a recruit (a footman with a name only) and a hero on the paladin; it also
            // has H001 and hfoo, which the map's Arthas and StrongFootman declare. The skin file holds the names.
            using var folders = new TestFolders();
            var map = folders.TemporaryFolderOutsideRepository();
            var main = new UnitObjectData(ObjectDataFormatVersion.v3);
            main.BaseUnits.Add(WorldEditorObject("hkni", string.Empty, Mod("uhpm", 500)));
            main.BaseUnits.Add(WorldEditorObject("hfoo", string.Empty, Mod("uhpm", 1)));
            main.NewUnits.Add(WorldEditorObject("hfoo", "h00R"));
            main.NewUnits.Add(WorldEditorObject("Hpal", "H00A", Mod("uhpm", 200), Mod("umpi", 1000)));
            main.NewUnits.Add(WorldEditorObject("Hpal", "H001", Mod("uhpm", 1)));
            var skin = new UnitObjectData(ObjectDataFormatVersion.v3);
            skin.BaseUnits.Add(WorldEditorObject("hkni", string.Empty, Mod("unam", "Edited Knight")));
            skin.NewUnits.Add(WorldEditorObject("hfoo", "h00R", Mod("unam", "Recruit")));
            skin.NewUnits.Add(WorldEditorObject("Hpal", "H001", Mod("unam", "Old name")));
            WriteUnits(Path.Combine(map, "war3map.w3u"), main);
            WriteUnits(Path.Combine(map, "war3mapSkin.w3u"), skin);

            var values = TypeValuesOf(ObjectLua.Values(compiled.Script, compiled.Declarations, NamedIds(), compiled.Standard, null, MapTypes.Read(map, null, compiled.Standard)));

            Assert.Contains($"[{ObjectLua.FourCC("hkni")}] = {{ [\"uhpm\"] = 500, [\"umpi\"] = 0, [\"unam\"] = \"Edited Knight\" }}", values);
            // A new type reads its base's standard values where it changes none: the standard footman's, not StrongFootman's.
            Assert.Contains($"[{ObjectLua.FourCC("h00R")}] = {{ [\"uhpm\"] = 420, [\"umpi\"] = 0, [\"unam\"] = \"Recruit\" }}", values);
            // The hero's life is its total, 200 + 22 * 25; its mana starts at most at its most mana, 0 + 17 * 15.
            Assert.Contains($"[{ObjectLua.FourCC("H00A")}] = {{ [\"uhpm\"] = 750, [\"umpi\"] = 255, [\"unam\"] = \"Paladin\" }}", values);
            // The map's classes win over World Editor's objects of their ids.
            Assert.Contains($"[{ObjectLua.FourCC("H001")}] = {{ [\"uhpm\"] = 1300, [\"umpi\"] = 255, [\"unam\"] = \"Arthas\" }}", values);
            Assert.Contains($"[{ObjectLua.FourCC("hfoo")}] = {{ [\"uhpm\"] = 600, [\"umpi\"] = 0, [\"unam\"] = \"Footman\" }}", values);
            Assert.Contains($"[{ObjectLua.FourCC("hpea")}] = {{ [\"uhpm\"] = 240, [\"umpi\"] = 0, [\"unam\"] = \"Peasant\" }}", values);
        }

        [Fact]
        public void type_values_read_world_editor_items_and_reals()
        {
            // Fields of a kind the Objects map does not read: an item's charges, a unit's scale (a real in the object file).
            using var folders = new TestFolders();
            var map = folders.TemporaryFolderOutsideRepository();
            var units = new UnitObjectData(ObjectDataFormatVersion.v3);
            units.NewUnits.Add(WorldEditorObject("hkni", "h00S"));
            var unitSkin = new UnitObjectData(ObjectDataFormatVersion.v3);
            unitSkin.NewUnits.Add(WorldEditorObject("hkni", "h00S", new SimpleObjectDataModification { Id = IdOf("usca"), Type = ObjectDataType.Real, Value = 2f }));
            var items = new ItemObjectData(ObjectDataFormatVersion.v3);
            items.BaseItems.Add(WorldEditorObject("ratf", string.Empty, Mod("iuse", 3)));
            items.NewItems.Add(WorldEditorObject("ratf", "I000"));
            WriteUnits(Path.Combine(map, "war3map.w3u"), units);
            WriteUnits(Path.Combine(map, "war3mapSkin.w3u"), unitSkin);
            using (var stream = File.Create(Path.Combine(map, "war3map.w3t")))
            using (var writer = new BinaryWriter(stream, Encoding.UTF8))
            {
                writer.Write(items);
            }

            var fields = new Dictionary<string, IReadOnlyList<string>>(StringComparer.Ordinal) { ["unit"] = new[] { "usca" }, ["item"] = new[] { "iuse" } };
            var declarations = compiled.Declarations with { TypeFields = fields };

            var values = TypeValuesOf(ObjectLua.Values(compiled.Script, declarations, NamedIds(), compiled.Standard, null, MapTypes.Read(map, null, compiled.Standard)));

            Assert.Contains($"[{ObjectLua.FourCC("h00S")}] = {{ [\"usca\"] = 2.0 }}", values);
            Assert.Contains($"[{ObjectLua.FourCC("ratf")}] = {{ [\"iuse\"] = 3 }}", values);
            Assert.Contains($"[{ObjectLua.FourCC("I000")}] = {{ [\"iuse\"] = 0 }}", values);
        }

        [Fact]
        public void type_values_read_world_editors_own_save_with_its_trigger_strings()
        {
            // World Editor keeps the new peasant's name in war3map.wts, as TRIGSTR_012.
            var fixture = TestFolders.Fixture("WorldEditorObjects");
            TriggerStrings strings;
            using (var reader = new StreamReader(Path.Combine(fixture, "war3map.wts")))
            {
                strings = reader.ReadTriggerStrings();
            }

            var types = MapTypes.Read(fixture, strings, compiled.Standard);

            var values = TypeValuesOf(ObjectLua.Values(compiled.Script, compiled.Declarations, NamedIds(), compiled.Standard, null, types));
            Assert.Contains($"[{ObjectLua.FourCC("h000")}] = {{ [\"uhpm\"] = 240, [\"umpi\"] = 0, [\"unam\"] = \"Unit\" }}", values);
            var item = Assert.Single(types, one => one.Id == "I000");
            Assert.Equal(("item", "ratf", (object?)"Item"), (item.Kind, item.Root, item.Changed["unam"]));
        }

        [Fact]
        public void a_trigger_string_reference_to_no_string_stays_as_it_is()
        {
            using var folders = new TestFolders();
            var map = folders.TemporaryFolderOutsideRepository();
            var skin = new UnitObjectData(ObjectDataFormatVersion.v3);
            skin.NewUnits.Add(WorldEditorObject("hfoo", "h00R", Mod("unam", "TRIGSTR_999")));
            WriteUnits(Path.Combine(map, "war3mapSkin.w3u"), skin);
            var strings = new TriggerStrings();
            strings.Strings.Add(new TriggerString { Key = 12, Value = "Unit" });

            var recruit = Assert.Single(MapTypes.Read(map, strings, compiled.Standard));

            Assert.Equal("TRIGSTR_999", recruit.Changed["unam"]);
        }

        [Fact]
        public void a_standard_class_reads_its_type_as_world_editor_changed_it()
        {
            // The map's code names Rifleman, and Paladin and Footman as bases of its classes; World Editor changed all three types.
            using var folders = new TestFolders();
            var map = folders.TemporaryFolderOutsideRepository();
            var main = new UnitObjectData(ObjectDataFormatVersion.v3);
            main.BaseUnits.Add(WorldEditorObject("hrif", string.Empty, Mod("uhpm", 777)));
            main.BaseUnits.Add(WorldEditorObject("Hpal", string.Empty, Mod("uhpm", 300)));
            main.BaseUnits.Add(WorldEditorObject("hfoo", string.Empty, Mod("uhpm", 1)));
            var skin = new UnitObjectData(ObjectDataFormatVersion.v3);
            skin.BaseUnits.Add(WorldEditorObject("hrif", string.Empty, Mod("unam", "Edited Rifleman")));
            WriteUnits(Path.Combine(map, "war3map.w3u"), main);
            WriteUnits(Path.Combine(map, "war3mapSkin.w3u"), skin);

            var values = CompiledObjects.ValuesOf(ObjectLua.Values(compiled.Script, compiled.Declarations, NamedIds(), compiled.Standard, null, MapTypes.Read(map, null, compiled.Standard)));

            string Of(string name) => Regex.Match(values, "\\[\"" + Regex.Escape(name) + "\"\\] = \\{[^\\n]*").Value;
            Assert.Contains("[\"uhpm\"] = 777", Of("Moonwell.Objects.Human.Units.Rifleman"));
            Assert.Contains("[\"unam\"] = \"Edited Rifleman\"", Of("Moonwell.Objects.Human.Units.Rifleman"));
            // The hero's total: 300 + 22 * 25.
            Assert.Contains("[\"uhpm\"] = 850", Of("Moonwell.Objects.Human.Units.Paladin"));
            // A class based on the paladin has the standard paladin's values, which Arthas doubles: (100 + 22 * 25) * 2.
            Assert.Contains("[\"uhpm\"] = 1300", Of("Objects.Arthas"));
            // StrongFootman replaces the footman, so World Editor's change to it is not in the map.
            Assert.Contains("[\"uhpm\"] = 420", Of("Moonwell.Objects.Human.Units.Footman"));
        }

        [Fact]
        public void a_map_that_does_not_read_default_writes_no_type_values()
        {
            using var folders = new TestFolders();
            var map = folders.TemporaryFolderOutsideRepository();
            var main = new UnitObjectData(ObjectDataFormatVersion.v3);
            main.NewUnits.Add(WorldEditorObject("hfoo", "h00R", Mod("uhpm", 500)));
            WriteUnits(Path.Combine(map, "war3map.w3u"), main);
            var declarations = compiled.Declarations with { TypeFields = new Dictionary<string, IReadOnlyList<string>>(StringComparer.Ordinal) };

            var script = ObjectLua.Values(compiled.Script, declarations, NamedIds(), compiled.Standard, null, MapTypes.Read(map, null, compiled.Standard));

            Assert.DoesNotContain("MoonwellTypeValues = {", CompiledObjects.ValuesOf(script));
        }

        [Fact]
        public void standard_class_used_in_code_is_defined()
        {
            // Footman is also the base of declared classes; Rifleman is used only inside a reaction.
            foreach (var (name, id) in new[] { ("Footman", "hfoo"), ("Rifleman", "hrif") })
            {
                var full = "Moonwell.Objects.Human.Units." + name;
                Assert.Contains($"namespace.class(\"{name}\"", compiled.Script);
                Assert.Contains(full, compiled.InitTypes());
                Assert.Contains($"global::{full}>(global::Moonwell.ObjectTypes.FourCC(\"{id}\")", compiled.Registry);
            }

            using var lua = compiled.Start();
            Assert.True(Truth(lua, "local made = Moonwell.ObjectTypes.UnitMaker(Moonwell.ObjectTypes.FourCC('hrif'))(); return System.is(made, Moonwell.Objects.Human.Units.Rifleman)"));
            Assert.Equal(
                "hrif",
                Query(lua, "local id = Moonwell.ObjectTypes.IdOf(Moonwell.Objects.Human.Units.Rifleman); return string.char(id >> 24, (id >> 16) & 255, (id >> 8) & 255, id & 255)"));
        }

        [Fact]
        public void setter_does_nothing_in_lua()
        {
            var paladin = int.Parse(compiled.Standard.Value("Hpal", "ugol", 0, null)!, CultureInfo.InvariantCulture);
            using var lua = compiled.Start();

            // Arthas's constructor runs GoldCost = GoldCost * 2: the value read is the declared one, and stays it.
            Assert.Equal(
                (2 * paladin).ToString(CultureInfo.InvariantCulture),
                Query(lua, "local a = Objects.Arthas(); return tostring(a:getGoldCost())"));
            Assert.Equal(
                (2 * paladin).ToString(CultureInfo.InvariantCulture),
                Query(lua, "local a = Objects.Arthas(); a:setGoldCost(5); return tostring(MoonwellObjectValues['Objects.Arthas']['ugol'])"));
        }

        [Fact]
        public void constructor_with_levels_per_level_and_set_runs()
        {
            using var lua = compiled.Start();

            // Tuned's constructor runs Firebolt's (Levels and PerLevel), then Set by level and Set of a whole field.
            // The damage is a float, so Lua prints it with its point (C18); the count is an int.
            Assert.Equal("100.0 250.0 300.0 3", Query(lua, "local t = Objects.Tuned(); local d = t:getDamage(); return d:get(1) .. ' ' .. d:get(2) .. ' ' .. d:get(3) .. ' ' .. d:getCount()"));
        }

        [Fact]
        public void one_value_for_every_level_runs_in_lua()
        {
            using var lua = compiled.Start();

            // Steady's constructor puts ManaCost = 60 and Duration = 30f: Levels' implicit conversion, whose value the setter drops.
            var errors = compiled.Run().Errors;
            Assert.True(errors.Count == 0, string.Join(" | ", errors));
            Assert.True(Truth(lua, "return System.is(Moonwell.ObjectTypes.AbilityMaker(Moonwell.ObjectTypes.FourCC('A007'))(), Objects.Steady)"));
        }

        [Fact]
        public void a_constructor_reads_the_default_of_a_class_registered_after_it_in_lua()
        {
            using var lua = compiled.Start();

            // Recruiter is registered before Veteran, and its constructor reads Veteran's default object.
            var errors = compiled.Run().Errors;
            Assert.True(errors.Count == 0, string.Join(" | ", errors));
            Assert.True(Truth(lua, "return System.is(Moonwell.Widget.Default(Objects.Veteran), Objects.Veteran)"));
            Assert.Equal("255", Query(lua, "return tostring(MoonwellObjectValues['Objects.Recruiter']['ugol'])"));
        }

        [Fact]
        public void every_list_call_runs_in_lua()
        {
            using var lua = compiled.Start();

            // Drilled's constructor calls Set with one to eight classes, Replace, Add and Remove; in the game they do nothing.
            // The list the build wrote: Firebolt, Elemental, Defend.
            Assert.Equal("3", Query(lua, "return tostring(Objects.Drilled():getAbilities():getCount())"));
        }

        [Fact]
        public void lists_and_references_read_in_lua()
        {
            using var lua = compiled.Start();

            Assert.True(Truth(lua, "return Objects.Duelist():getAbilities():Contains(Objects.Firebolt)"));
            Assert.True(Truth(lua, "return Objects.Duelist():getAbilities():Contains(Moonwell.Objects.Human.Abilities.Defend)"));
            Assert.False(Truth(lua, "return Objects.Arthas():getHeroAbilities():Contains(Moonwell.Objects.Human.Abilities.DevotionAura)"));
            // Arthas's hero abilities hold the ids of the standard objects in the order the declaration left them.
            Assert.True(Truth(lua, "local list = Objects.Arthas():getHeroAbilities(); return list:getCount() == 3 and list[1] == Moonwell.ObjectTypes.FourCC('AHhb')"));
            Assert.True(Truth(lua, "return Objects.Duelist():getDefaultActiveAbility():Is(Objects.Firebolt)"));
            Assert.False(Truth(lua, "return Objects.Duelist():getDefaultActiveAbility():Is(Moonwell.Objects.Human.Abilities.Defend)"));
            Assert.Equal("3", Query(lua, "return tostring(Objects.Duelist():getAbilities():getCount())"));
        }

        [Fact]
        public void values_are_found_through_the_base_chain()
        {
            using var lua = compiled.Start();

            // A class with no entry of its own reads the entry of the nearest base that has one; a nested class's name has a +.
            lua.DoString("System.define('Test.Outer+Inner', { base = { Objects.Arthas } })");
            Assert.Equal(
                Query(lua, "return tostring(MoonwellObjectValues['Objects.Arthas']['ugol'])"),
                Query(lua, "return tostring(Test.Outer.Inner():getGoldCost())"));
        }

        [Fact]
        public void stubs_have_only_the_members_the_map_references()
        {
            // Arthas's constructor reads and sets GoldCost; no code names AnimationWalkSpeed.
            var hero = compiled.Declarations.Used.Single(one => one.FullName == "Moonwell.Objects.StandardHero");
            Assert.Contains(hero.Fields, field => field.Property == "GoldCost");
            Assert.DoesNotContain(hero.Fields, field => field.Property == "AnimationWalkSpeed");

            Assert.Contains("    public abstract class StandardHero : global::Moonwell.Hero\n", compiled.Stubs);
            Assert.Contains("        public int GoldCost\n        {\n            get { return default; }\n            protected set { }\n        }\n", compiled.Stubs);
            Assert.Contains(
                "namespace Moonwell.Objects\n{\n"
                + "    /// @CSharpLua.Ignore\n    public static partial class Human\n    {\n"
                + "        /// @CSharpLua.Ignore\n        public static partial class Units\n        {\n",
                compiled.Stubs);
            Assert.Contains("            /// @CSharpLua.Ignore\n            public class Paladin : global::Moonwell.Objects.StandardHero\n", compiled.Stubs);
            Assert.DoesNotContain("AnimationWalkSpeed", compiled.Stubs);
            Assert.DoesNotContain("getAnimationWalkSpeed", compiled.Script);
            Assert.DoesNotContain("Archmage", compiled.Stubs);
        }

        [Fact]
        public void stubs_have_only_the_enums_the_map_names()
        {
            // The enums' own file is left out of CSharp.lua's compile, which would write a blank line for each of them into every map: the stubs carry the ones the code names.
            Assert.Contains("    /// @CSharpLua.Ignore\n    public enum DeathType\n    {\n", compiled.Stubs);
            Assert.Contains("        CanRaiseDoesNotDecay = 1,\n", compiled.Stubs);
            Assert.Contains("    /// @CSharpLua.Ignore\n    [global::System.Flags]\n    public enum Targets\n    {\n", compiled.Stubs);
            Assert.Contains("    public enum ArmorType\n", compiled.Stubs);
            Assert.DoesNotContain("public enum AiBuffer", compiled.Stubs);
            Assert.DoesNotContain("public enum AbilityModifierStats", compiled.Stubs);
            Assert.DoesNotContain("BeyondLimitsAttribute", compiled.Stubs);

            // The attack and defense types are core Moonwell's, compiled with it: no stub.
            Assert.DoesNotContain("enum AttackType", compiled.Stubs);
            Assert.DoesNotContain("enum DefenseType", compiled.Stubs);
            Assert.Contains("public global::Moonwell.AttackType AttackType\n", compiled.Stubs);
        }

        [Fact]
        public void only_enums_with_game_values_are_choices_to_stub()
        {
            // A public helper enum in Moonwell.Objects without [GameValue] members is compiled by CSharp.lua like any code, so a stub would be a second one.
            var module = AssemblyBuilder.DefineDynamicAssembly(new AssemblyName("Helpers"), AssemblyBuilderAccess.Run).DefineDynamicModule("Helpers");
            var attribute = module.DefineType("Moonwell.Objects.GameValueAttribute", TypeAttributes.Public, typeof(Attribute));
            var constructor = attribute.DefineConstructor(MethodAttributes.Public, CallingConventions.Standard, new[] { typeof(string) });
            var il = constructor.GetILGenerator();
            il.Emit(OpCodes.Ldarg_0);
            il.Emit(OpCodes.Call, typeof(Attribute).GetConstructor(BindingFlags.NonPublic | BindingFlags.Instance, Type.EmptyTypes)!);
            il.Emit(OpCodes.Ret);
            var gameValue = attribute.CreateType().GetConstructor(new[] { typeof(string) })!;

            var choice = module.DefineEnum("Moonwell.Objects.Colour", TypeAttributes.Public, typeof(int));
            choice.DefineLiteral("Red", 1).SetCustomAttribute(new CustomAttributeBuilder(gameValue, new object[] { "red" }));
            var helper = module.DefineEnum("Moonwell.Objects.Helper", TypeAttributes.Public, typeof(int));
            helper.DefineLiteral("One", 1);
            var elsewhere = module.DefineEnum("Other.Colour", TypeAttributes.Public, typeof(int));
            elsewhere.DefineLiteral("Red", 1).SetCustomAttribute(new CustomAttributeBuilder(gameValue, new object[] { "red" }));

            var enums = new HashSet<Type>();
            foreach (var type in new[] { choice.CreateType(), helper.CreateType(), elsewhere.CreateType() })
            {
                ObjectStubs.AddEnums(type, enums);
            }

            Assert.Equal(new[] { "Moonwell.Objects.Colour" }, enums.Select(type => type.FullName));
        }

        [Fact]
        public void every_type_the_library_leaves_out_of_csharp_lua_has_a_stub_or_is_a_choice()
        {
            // A new attribute or exception in a file inside #if !MOONWELL_LUA would be unknown to CSharp.lua unless the stubs carry it.
            var core = Path.GetFullPath(Path.Combine(AppContext.BaseDirectory, "..", "..", "..", "..", "..", "src", "Moonwell.Objects", "Core"));
            var left = new List<string>();
            foreach (var file in Directory.GetFiles(core, "*.cs"))
            {
                var text = File.ReadAllText(file);
                if (!text.StartsWith("#if !MOONWELL_LUA", StringComparison.Ordinal))
                {
                    continue;
                }

                foreach (Match match in Regex.Matches(text, @"^    public (?:sealed )?class (\w+)", RegexOptions.Multiline | RegexOptions.CultureInvariant))
                {
                    left.Add(match.Groups[1].Value);
                }
            }

            // GameValueAttribute marks the choices' members: no map has use for it.
            Assert.Equal(
                new[] { "GameValueAttribute" },
                left.Where(name => !ObjectStubs.IsIgnoredName(name)).OrderBy(name => name, StringComparer.Ordinal));
            Assert.Contains("FieldRuleException", left);
            Assert.Contains("RequiredAttribute", left);
        }

        [Fact]
        public void unused_generated_namespace_leaves_nothing_in_the_script()
        {
            Assert.DoesNotContain("Moonwell.Objects.Campaign.Units", compiled.Script);
            Assert.DoesNotContain("Moonwell.Objects.Other.Abilities", compiled.Script);
        }

        [Fact]
        public void unused_standard_class_absent()
        {
            // The table by type id has every type's name.
            var types = compiled.Script.LastIndexOf("MoonwellTypeValues = {", StringComparison.Ordinal);
            Assert.DoesNotContain("Archmage", compiled.Script.Substring(0, types));
            Assert.DoesNotContain("Archmage", compiled.Registry);
        }

        [Fact]
        public void init_list_after_base()
        {
            var types = compiled.InitTypes();
            var pairs = new List<(string Class, string Base)>();
            foreach (var used in compiled.Declarations.Used)
            {
                pairs.Add((used.FullName, used.BaseFullName));
            }

            pairs.Add(("Objects.Arthas", "Moonwell.Objects.Human.Units.Paladin"));
            pairs.Add(("Objects.DarkArthas", "Objects.Arthas"));
            pairs.Add(("Objects.Duelist", "Moonwell.Objects.Human.Units.Footman"));
            pairs.Add(("Objects.Tuned", "Objects.Firebolt"));
            foreach (var (name, parent) in pairs)
            {
                var at = types.IndexOf(name);
                Assert.True(at >= 0, name + " is not in the init list");
                Assert.True(types.IndexOf(parent) >= 0 && types.IndexOf(parent) < at, $"{name} comes before its base {parent}");
            }

            Assert.Contains("Moonwell.Objects.StandardHero", types);
        }

        [Theory]
        [InlineData(false)]
        [InlineData(true)]
        public void values_only_read_fields(bool release)
        {
            // A release build writes no comment before the compiled code; the library's own getters stay out of the scan.
            var values = CompiledObjects.ValuesOf(release ? compiled.ReleaseScript : compiled.Script);
            var arthas = Regex.Match(values, "\\[\"Objects\\.Arthas\"\\] = \\{[^\\n]*").Value;

            Assert.Contains("[\"ugol\"] = ", arthas);
            // No code reads Arthas's AI placement radius.
            Assert.DoesNotContain("[\"uabr\"]", values);
        }

        [Fact]
        public void values_are_found_when_the_code_holds_the_end_of_the_library_as_text()
        {
            // The text of the library's last line, in a string of the map's code, is not where the library ends.
            var ids = new Dictionary<string, string>(compiled.Declarations.StandardClassIds, StringComparer.Ordinal);
            foreach (var entry in compiled.Ids)
            {
                ids[entry.Key] = entry.Value;
            }

            var script = compiled.Script + "\nlocal text = \"local endOfMoonwellObjects = true\"\n";

            var values = CompiledObjects.ValuesOf(ObjectLua.Values(script, compiled.Declarations, ids, compiled.Standard, null, Array.Empty<MapType>()));

            Assert.Contains("[\"ugol\"] = ", values);
        }

        [Theory]
        [InlineData("int", "12", "12")]
        [InlineData("int", "45.5", "45")]
        [InlineData("int", "-45.5", "-45")]
        [InlineData("int", "#VALUE!", "0")]
        [InlineData("int", "MBUR", "0")]
        [InlineData("int", "ap04", "0")]
        [InlineData("int", null, "0")]
        [InlineData("float", "0.25", "0.25")]
        [InlineData("float", "5", "5.0")]
        [InlineData("float", "#VALUE!", "0.0")]
        [InlineData("float", "ap04", "0.0")]
        public void standard_text_that_is_not_a_number_reads_as_the_net_side_does(string type, string? text, string expected)
        {
            // The same texts as DeclaredTests.parse_int and parse_float_takes_zero_for_what_is_not_a_number: both sides agree.
            Assert.Equal(expected, FieldValues.Scalar(type, text));
        }

        [Theory]
        [InlineData(5f, "5.0")]
        [InlineData(-2f, "-2.0")]
        [InlineData(1.5f, "1.5")]
        [InlineData(1e20f, "1E+20")]
        public void whole_number_float_values_keep_a_point(float value, string expected)
        {
            // Lua reads 5 as an integer, which does integer arithmetic (C18).
            Assert.Equal(expected, FieldValues.Scalar("float", value));
        }

        [Fact]
        public void unset_string_reads_empty_in_lua()
        {
            using var lua = compiled.Start();

            // Arthas's description has no value; it reads as the empty text, in compiled code (Description + "!") and in plain Lua.
            Assert.Equal("!", Query(lua, "return Objects.Report.Described(Objects.Arthas())"));
            Assert.True(Truth(lua, "return Objects.Arthas():getDescription() == ''"));
            Assert.Equal("!", Query(lua, "return Objects.Arthas():getDescription() .. '!'"));
        }

        [Fact]
        public void choices_read_in_lua_as_their_numbers()
        {
            using var lua = compiled.Start();

            // The mage's declared choices, the footman's standard ones (Metal armor, targeted as ground) and the zap's levels.
            Assert.Equal("6", Query(lua, "return tostring(Objects.Mage():getAttackType())"));
            Assert.Equal("6", Query(lua, "return tostring(Objects.Mage():getDefenseType())"));
            Assert.Equal("33", Query(lua, "return tostring(Objects.Mage():getAttack1TargetsAllowed())"));
            Assert.Equal("1", Query(lua, "return tostring(Objects.Mage():getArmorType())"));
            Assert.Equal("64", Query(lua, "return tostring(Objects.Mage():getTargetedAs())"));
            Assert.Equal("yes yes 1048640", Query(lua, "return Objects.Report.Chosen(Objects.Mage(), Objects.Zap())"));

            // The enums are ignored: CSharp.lua writes their members as numbers (a debug build names them in a comment) and no Lua of their own.
            Assert.Contains("33 --[[Moonwell.Objects.Targets.Air | Moonwell.Objects.Targets.Enemies]]", compiled.Script);
            // Report's enum methods name the type (MoonwellObjects.DeathType, a nil in the game); nothing defines it.
            Assert.DoesNotMatch(@"(?<![gs]et|MoonwellObjects\.)DeathType", compiled.ReleaseScript);
            Assert.DoesNotContain("Moonwell.Objects.Targets", compiled.ReleaseScript);
        }

        [Fact]
        public void enum_methods_on_a_choice_in_game_code()
        {
            using var lua = compiled.Start();

            // The enums are ignored, so the Lua has no type of theirs: a name is the number (on .NET it is "Magic"), and
            // Enum.GetValues stops. HasFlag needs only the numbers. MW0010 rejects the first two in map code.
            Assert.Equal("1", Outcome(lua, "return Objects.Report.Named(Objects.Mage())"));
            Assert.Equal("death 1 1", Outcome(lua, "return Objects.Report.Joined(Objects.Mage())"));
            Assert.Equal("True", Outcome(lua, "return Objects.Report.Flagged(Objects.Mage())"));
            Assert.StartsWith("error: ", Outcome(lua, "return Objects.Report.Counted()"));
        }

        [Fact]
        public void a_core_choice_has_its_names_in_lua()
        {
            using var lua = compiled.Start();

            // AttackType and DefenseType are core Moonwell's: CSharp.lua compiles them, so MW0010 lets their names be read.
            Assert.Equal("Magic Divine Hero", Outcome(lua, "return Objects.Report.CoreNamed(Objects.Mage())"));
        }

        // What a chunk returns, or "error: " and the error it raises.
        private static string Outcome(NLua.Lua lua, string chunk)
        {
            try
            {
                return Query(lua, chunk);
            }
            catch (NLua.Exceptions.LuaException error)
            {
                return "error: " + error.Message;
            }
        }

        [Fact]
        public void ctor_runs_runtime_base()
        {
            using var lua = compiled.Start();

            Assert.Equal("1", Query(lua, "local a = Objects.Arthas(); return tostring(a.Components:getCount())"));
            Assert.True(Truth(lua, "local a = Objects.Arthas(); return System.is(a.Components:get(0), Objects.Rage)"));
        }

        [Fact]
        public void registry_written()
        {
            var registry = compiled.Registry;
            var ids = compiled.Ids;

            Assert.StartsWith("// Generated by Moonwell's build.", registry);
            Assert.Contains("namespace Moonwell.Generated", registry);
            Assert.Contains("public static void Register()", registry);
            Assert.Contains($"global::Moonwell.ObjectTypes.Unit<global::Objects.Arthas>(global::Moonwell.ObjectTypes.FourCC(\"{ids["Objects.Arthas"]}\"), () => new global::Objects.Arthas());", registry);
            Assert.Contains($"global::Moonwell.ObjectTypes.Ability<global::Objects.Firebolt>(global::Moonwell.ObjectTypes.FourCC(\"{ids["Objects.Firebolt"]}\"), () => new global::Objects.Firebolt());", registry);
            Assert.Contains($"global::Moonwell.ObjectTypes.Item<global::Objects.Claws>(global::Moonwell.ObjectTypes.FourCC(\"{ids["Objects.Claws"]}\"), () => new global::Objects.Claws());", registry);
            Assert.Contains($"global::Moonwell.ObjectTypes.Destructable<global::Objects.Gate>(global::Moonwell.ObjectTypes.FourCC(\"{ids["Objects.Gate"]}\"), () => new global::Objects.Gate());", registry);
            Assert.Contains($"global::Moonwell.ObjectTypes.Id<global::Objects.Rock>(global::Moonwell.ObjectTypes.FourCC(\"{ids["Objects.Rock"]}\"));", registry);
            Assert.Contains($"global::Moonwell.ObjectTypes.Id<global::Objects.Forge>(global::Moonwell.ObjectTypes.FourCC(\"{ids["Objects.Forge"]}\"));", registry);
            Assert.Contains($"global::Moonwell.ObjectTypes.Id<global::Objects.Burn>(global::Moonwell.ObjectTypes.FourCC(\"{ids["Objects.Burn"]}\"));", registry);
            Assert.Contains($"global::Moonwell.ObjectTypes.BuffDisplay<global::Objects.Burn>(global::Moonwell.ObjectTypes.FourCC(\"{ids["Objects.Burn"]}\"), global::Moonwell.ObjectTypes.FourCC(\"{ids["Objects.Burn#carrier"]}\"));", registry);

            // StrongFootman replaces Footman: it is made at hfoo, and Footman only has the id.
            Assert.Contains("global::Moonwell.ObjectTypes.Unit<global::Objects.StrongFootman>(global::Moonwell.ObjectTypes.FourCC(\"hfoo\"), () => new global::Objects.StrongFootman());", registry);
            Assert.DoesNotContain("Unit<global::Moonwell.Objects.Human.Units.Footman>", registry);
            Assert.Contains("global::Moonwell.ObjectTypes.Id<global::Moonwell.Objects.Human.Units.Footman>(global::Moonwell.ObjectTypes.FourCC(\"hfoo\"));", registry);

            // The kind bases have no object.
            Assert.DoesNotContain("StandardHero", registry);

            Assert.Contains("global::Moonwell.Runtime.Start(new global::Objects.ObjectsMap(), global::Moonwell.Generated.ObjectRegistry.Register);", compiled.Entry);
        }

        [Fact]
        public void registrations_run_in_lua()
        {
            using var lua = compiled.Start();

            Assert.True(Truth(lua, "local made = Moonwell.ObjectTypes.UnitMaker(Moonwell.ObjectTypes.FourCC('hfoo'))(); return System.is(made, Objects.StrongFootman)"));
            Assert.Equal(compiled.Ids["Objects.Burn"], Query(lua, "local id = Moonwell.ObjectTypes.IdOf(Objects.Burn); return string.char(id >> 24, (id >> 16) & 255, (id >> 8) & 255, id & 255)"));
        }

        [Fact]
        public void a_map_without_object_classes_loads()
        {
            // A map that references Moonwell.Objects loads its ignored files' preambles, even with no class used.
            var definitions = ObjectLua.Definitions(Array.Empty<UsedClass>());

            using var lua = new NLua.Lua();
            lua.DoString("Moonwell = {}");
            lua.DoString(definitions);
            Assert.Equal("table", Query(lua, "return type(Moonwell.Objects.Levels_1(1))"));
            Assert.Equal("nil", Query(lua, "return type(Moonwell.Objects.Levels_1(1).op_Implicit(60))"));
            Assert.Equal("table", Query(lua, "return type(Moonwell.Objects.ObjectList_1(1))"));
            Assert.Equal("table", Query(lua, "return type(Moonwell.Objects.ObjectRef_1(1))"));
        }

        // The ids the build gives Values: the standard classes' and the declared classes'.
        private Dictionary<string, string> NamedIds()
        {
            var ids = new Dictionary<string, string>(compiled.Declarations.StandardClassIds, StringComparer.Ordinal);
            foreach (var entry in compiled.Ids)
            {
                ids[entry.Key] = entry.Value;
            }

            return ids;
        }

        // The table of the types' values appended to a script.
        private static string TypeValuesOf(string script)
        {
            var at = script.LastIndexOf("MoonwellTypeValues = {", StringComparison.Ordinal);
            Assert.True(at >= 0, "the script has no table of the types' values");
            return script.Substring(at);
        }

        private static SimpleObjectModification WorldEditorObject(string oldId, string newId, params SimpleObjectDataModification[] modifications)
        {
            var one = new SimpleObjectModification { OldId = IdOf(oldId), NewId = newId.Length == 0 ? 0 : IdOf(newId) };
            one.Unk.Add(0);
            one.Modifications.AddRange(modifications);
            return one;
        }

        private static SimpleObjectDataModification Mod(string field, object value)
        {
            return new SimpleObjectDataModification { Id = IdOf(field), Type = value is int ? ObjectDataType.Int : ObjectDataType.String, Value = value };
        }

        private static int IdOf(string text)
        {
            return BitConverter.ToInt32(Encoding.ASCII.GetBytes(text));
        }

        private static void WriteUnits(string path, UnitObjectData data)
        {
            using var stream = File.Create(path);
            using var writer = new BinaryWriter(stream, Encoding.UTF8);
            writer.Write(data);
        }

        private static bool Truth(NLua.Lua lua, string chunk)
        {
            return (bool)lua.DoString(chunk)[0];
        }

        private static string Query(NLua.Lua lua, string chunk)
        {
            return Convert.ToString(lua.DoString(chunk)[0], CultureInfo.InvariantCulture)!;
        }

        /// <summary>The Objects fixture, copied and compiled to Lua with its objects' library, values and registry.</summary>
        public sealed class CompiledObjects : IDisposable
        {
            private static readonly string[] Kinds = { "unit", "item", "destructable", "doodad", "ability", "buff", "upgrade" };

            private readonly TestFolders folders = new TestFolders();

            public CompiledObjects()
            {
                var folder = folders.TemporaryCopyOf("Objects");

                var project = MapProject.Find(folder);
                var assembly = DotnetBuild.Run(project, false, TextWriter.Null);
                Standard = StandardData.Read(Path.Combine(Path.GetDirectoryName(assembly)!, "Moonwell.Objects.dll"));
                var settings = MapSettingsReader.Read(assembly, project);
                var map = SourceMap.Open(project, settings, TextWriter.Null);
                Declarations = Build.Declarations.Read(assembly, project, Standard, null);
                var standardIds = new HashSet<string>();
                foreach (var kind in Kinds)
                {
                    standardIds.UnionWith(Standard.Ids(kind));
                }

                Ids = ObjectIds.Assign(Declarations.Objects, standardIds, new HashSet<string>());
                Registry = File.ReadAllText(RegistryWriter.Write(project, Declarations.Used, Declarations.Objects, Ids));
                Entry = File.ReadAllText(EntryWriter.Write(project, settings, true));
                Script = LuaCompiler.Compile(project, map, assembly, false, Declarations, Ids, Standard, null);
                Stubs = File.ReadAllText(Path.Combine(project.GeneratedFolder, "ObjectStubs.g.cs"));
                ReleaseScript = LuaCompiler.Compile(project, SourceMap.Open(project, settings, TextWriter.Null), assembly, true, Declarations, Ids, Standard, null);
            }

            public StandardData Standard { get; }

            public DeclarationSet Declarations { get; }

            public IReadOnlyDictionary<string, string> Ids { get; }

            public string Registry { get; }

            public string Entry { get; }

            public string Script { get; }

            /// <summary>The stubs of the used classes CSharp.lua compiled.</summary>
            public string Stubs { get; }

            /// <summary>The script of a release build, which has no comments.</summary>
            public string ReleaseScript { get; }

            /// <summary>The names in System.init's types list, in order.</summary>
            public List<string> InitTypes()
            {
                var start = Script.IndexOf("System.init({", StringComparison.Ordinal);
                var open = Script.IndexOf("types = {", start, StringComparison.Ordinal);
                var close = Script.IndexOf('}', open + "types = {".Length);
                return Regex.Matches(Script.Substring(open, close - open), "\"([^\"]+)\"").Select(match => match.Groups[1].Value).ToList();
            }

            /// <summary>The values table appended to a script.</summary>
            public static string ValuesOf(string script)
            {
                var at = script.LastIndexOf("MoonwellObjectValues = {", StringComparison.Ordinal);
                Assert.True(at >= 0, "the script has no values table");
                return script.Substring(at);
            }

            /// <summary>
            /// Loads the script with the game's globals stubbed, runs main (which defines the classes and starts the runtime), then runs the
            /// timers the start asked for once: the runtime's setup registers the objects and starts the game.
            /// </summary>
            public NLua.Lua Start()
            {
                var lua = new NLua.Lua();
                lua.DoString("lines = {}; pending = {}");
                lua.DoString("print = function(...) end");
                lua.DoString("setmetatable(_G, { __index = function(_, name) return function() return nil end end })");
                lua.DoString("bj_MAX_PLAYER_SLOTS = 0");
                lua.DoString("TimerStart = function(timer, timeout, periodic, callback) pending[#pending + 1] = callback end");
                lua.DoString("DisplayTextToPlayer = function(p, x, y, text) lines[#lines + 1] = text end");
                lua.DoString("DisplayTimedTextToPlayer = function(p, x, y, duration, text) lines[#lines + 1] = 'error: ' .. text end");
                lua.DoString(Script);
                lua.DoString("main()");
                lua.DoString("local due = pending; pending = {}; for i = 1, #due do due[i]() end");
                return lua;
            }

            /// <summary>Starts the map and gives what it showed.</summary>
            public LuaRun Run()
            {
                using var lua = Start();
                var count = Convert.ToInt32(lua.DoString("return #lines")[0], CultureInfo.InvariantCulture);
                var lines = new List<string>();
                for (var i = 1; i <= count; i++)
                {
                    lines.Add((string)lua.DoString($"return lines[{i}]")[0]);
                }

                return new LuaRun(lines.Where(line => !line.StartsWith("error: ", StringComparison.Ordinal)).ToList(), lines.Where(line => line.StartsWith("error: ", StringComparison.Ordinal)).ToList());
            }

            public void Dispose()
            {
                folders.Dispose();
            }
        }

        /// <summary>What a run of the map showed: its lines, and the errors the runtime reported.</summary>
        public sealed record LuaRun(IReadOnlyList<string> Lines, IReadOnlyList<string> Errors);
    }
}
