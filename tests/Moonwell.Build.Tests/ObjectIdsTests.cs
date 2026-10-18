using System.Text;
using System.Text.Json;

namespace Moonwell.Build.Tests
{
    public sealed class ObjectIdsTests : IDisposable
    {
        private static readonly IReadOnlySet<string> None = new HashSet<string>();

        private readonly string folder = Path.Combine(Path.GetTempPath(), "moonwell-ids-" + Guid.NewGuid().ToString("N"));
        private readonly StringWriter log = new StringWriter { NewLine = "\n" };

        public ObjectIdsTests()
        {
            Directory.CreateDirectory(Path.Combine(folder, "map"));
            File.WriteAllBytes(Path.Combine(folder, "map", "war3map.w3i"), Array.Empty<byte>());
            File.WriteAllText(Path.Combine(folder, "Sample.csproj"), "<Project />");
        }

        public void Dispose()
        {
            Directory.Delete(folder, true);
        }

        [Fact]
        public void new_ids_by_kind()
        {
            var ids = Assign(
                Obj("Objects.Arthas", "hero"),
                Obj("Objects.Footman", "unit"),
                Obj("Objects.Bolt", "ability"),
                Obj("Objects.Sword", "item"),
                Obj("Objects.Burn", "buff"),
                Obj("Objects.Upgrade", "upgrade"),
                Obj("Objects.Gate", "destructable"),
                Obj("Objects.Bush", "doodad"));

            Assert.Equal("H001", ids["Objects.Arthas"]);
            Assert.Equal("h001", ids["Objects.Footman"]);
            Assert.Equal("A001", ids["Objects.Bolt"]);
            Assert.Equal("I001", ids["Objects.Sword"]);
            Assert.Equal("B001", ids["Objects.Burn"]);
            Assert.Equal("A002", ids["Objects.Burn#carrier"]);
            Assert.Equal("R001", ids["Objects.Upgrade"]);
            Assert.Equal("B001", ids["Objects.Gate"]);
            Assert.Equal("D001", ids["Objects.Bush"]);
            Assert.Equal(9, ids.Count);
        }

        [Fact]
        public void heroes_and_units_share_a_space_but_count_apart()
        {
            var ids = Assign(Obj("A.Hero", "hero"), Obj("B.Hero", "hero"), Obj("C.Unit", "unit"));

            Assert.Equal("H001", ids["A.Hero"]);
            Assert.Equal("H002", ids["B.Hero"]);
            Assert.Equal("h001", ids["C.Unit"]);
        }

        [Fact]
        public void counter_goes_from_digits_to_letters_to_the_next_digit()
        {
            var objects = new List<DeclaredObject>();
            for (var index = 0; index < 38; index++)
            {
                objects.Add(Obj("Objects.U" + index.ToString("D2"), "unit"));
            }

            var ids = Assign(objects.ToArray());

            Assert.Equal("h009", ids["Objects.U08"]);
            Assert.Equal("h00A", ids["Objects.U09"]);
            Assert.Equal("h00Z", ids["Objects.U34"]);
            Assert.Equal("h010", ids["Objects.U35"]);
            Assert.Equal("h011", ids["Objects.U36"]);
        }

        [Fact]
        public void kept_on_rebuild()
        {
            Assign(Obj("Objects.A", "unit"), Obj("Objects.B", "unit"));

            var again = Assign(Obj("Objects.B", "unit"), Obj("Objects.C", "unit"), Obj("Objects.A", "unit"));

            Assert.Equal("h001", again["Objects.A"]);
            Assert.Equal("h002", again["Objects.B"]);
            Assert.Equal("h003", again["Objects.C"]);
        }

        [Fact]
        public void pin_wins()
        {
            Assign(Obj("Objects.A", "unit"));
            Assert.Equal("h001", ReadFile()["Objects.A"]);

            var ids = Assign(Obj("Objects.A", "unit", pin: "h0AB"));

            Assert.Equal("h0AB", ids["Objects.A"]);
            Assert.Equal("h0AB", ReadFile()["Objects.A"]);
        }

        [Fact]
        public void allocation_skips_source_map_ids()
        {
            var ids = Assign(
                None,
                new HashSet<string> { "h001", "h003" },
                None,
                Obj("Objects.A", "unit"),
                Obj("Objects.B", "unit"));

            Assert.Equal("h002", ids["Objects.A"]);
            Assert.Equal("h004", ids["Objects.B"]);
        }

        [Fact]
        public void skips_standard_ids()
        {
            var ids = Assign(
                new HashSet<string> { "A001", "A002" },
                None,
                None,
                Obj("Objects.A", "ability"));

            Assert.Equal("A003", ids["Objects.A"]);
        }

        [Fact]
        public void skips_placed_ids()
        {
            var ids = Assign(
                None,
                None,
                new HashSet<string> { "h001" },
                Obj("Objects.A", "unit"));

            Assert.Equal("h002", ids["Objects.A"]);
        }

        [Fact]
        public void skips_ids_of_gone_entries()
        {
            Assign(Obj("Objects.Old", "unit"));

            var ids = Assign(Obj("Objects.New", "unit"));

            Assert.Equal("h002", ids["Objects.New"]);
        }

        [Fact]
        public void rename_warns()
        {
            Assign(Obj("Objects.Old", "unit"));

            var ids = Assign(Obj("Objects.New", "unit"));

            Assert.Equal("h002", ids["Objects.New"]);
            Assert.Equal(
                "warning: Objects.Old was h001, and no class declares it now; if it was renamed to Objects.New, add [ObjectId(\"h001\")] to Objects.New\n",
                log.ToString());
        }

        [Fact]
        public void rename_reports_hero_and_unit_as_one_kind()
        {
            Assign(Obj("Objects.Old", "unit"));

            Assign(Obj("Objects.New", "hero"));

            Assert.Contains("if it was renamed to Objects.New", log.ToString());
        }

        [Fact]
        public void no_rename_report_when_nothing_is_new()
        {
            Assign(Obj("Objects.Old", "unit"));

            Assign();

            Assert.Equal(string.Empty, log.ToString());
        }

        [Fact]
        public void no_rename_report_when_the_new_class_is_of_another_kind()
        {
            Assign(Obj("Objects.Old", "unit"));

            var ids = Assign(Obj("Objects.Sword", "item"));

            Assert.Equal("I001", ids["Objects.Sword"]);
            Assert.Equal(string.Empty, log.ToString());
        }

        [Fact]
        public void no_rename_report_when_the_new_class_pins_the_old_id()
        {
            Assign(Obj("Objects.Old", "unit"));

            var ids = Assign(Obj("Objects.New", "unit", pin: "h001"));

            Assert.Equal("h001", ids["Objects.New"]);
            Assert.Equal(string.Empty, log.ToString());
        }

        [Fact]
        public void rename_of_placed_id_fails()
        {
            Assign(Obj("Objects.Old", "unit"));
            var before = File.ReadAllText(Path.Combine(folder, "ids.json"));

            var error = Assert.Throws<BuildException>(() => Assign(
                None,
                None,
                new HashSet<string> { "h001" },
                Obj("Objects.New", "unit")));

            Assert.Equal(
                "Objects.Old was h001, which the map places, and no class declares it now; if it was renamed to Objects.New, add [ObjectId(\"h001\")] to Objects.New",
                error.Message);
            Assert.Equal(before, File.ReadAllText(Path.Combine(folder, "ids.json")));
        }

        [Fact]
        public void gone_placed_id_without_a_new_class_is_a_warning()
        {
            Assign(Obj("Objects.Old", "unit"));

            var ids = Assign(None, None, new HashSet<string> { "h001" }, Obj("Objects.Other", "item"));

            Assert.Equal("I001", ids["Objects.Other"]);
            Assert.Equal("warning: Objects.Old was h001, which the map places, and no class declares it now\n", log.ToString());
        }

        [Fact]
        public void gone_id_the_map_does_not_place_is_no_warning_without_a_new_class()
        {
            Assign(Obj("Objects.Old", "unit"));

            Assign(Obj("Objects.Other", "item"));

            Assert.Equal(string.Empty, log.ToString());
        }

        [Fact]
        public void replaces_takes_standard_id()
        {
            var ids = Assign(Obj("Objects.Footman", "unit", root: "hfoo", replaces: true), Obj("Objects.Other", "unit"));

            Assert.Equal("hfoo", ids["Objects.Footman"]);
            Assert.Equal("h001", ids["Objects.Other"]);
            Assert.False(ReadFile().ContainsKey("Objects.Footman"));
        }

        [Fact]
        public void replaces_cannot_pin()
        {
            var error = Assert.Throws<BuildException>(
                () => Assign(Obj("Objects.Footman", "unit", root: "hfoo", replaces: true, pin: "h001")));

            Assert.Equal("Objects.Footman replaces hfoo and cannot pin an id", error.Message);
        }

        [Fact]
        public void gone_leaves_file()
        {
            Assign(Obj("Objects.A", "unit"), Obj("Objects.B", "buff"));
            Assert.Equal(3, ReadFile().Count);

            Assign(Obj("Objects.A", "unit"));

            Assert.Equal(new[] { "Objects.A" }, ReadFile().Keys);
        }

        [Fact]
        public void file_is_sorted()
        {
            Assign(Obj("Zed.Z", "unit"), Obj("Alpha.B", "buff"), Obj("alpha.a", "item"), Obj("Alpha.A", "hero"));

            var path = Path.Combine(folder, "ids.json");

            Assert.Equal(
                "{\n  \"Alpha.A\": \"H001\",\n  \"Alpha.B\": \"B001\",\n  \"Alpha.B#carrier\": \"A001\",\n  \"Zed.Z\": \"h001\",\n  \"alpha.a\": \"I001\"\n}\n",
                File.ReadAllText(path));
            Assert.False(File.ReadAllBytes(path).AsSpan().StartsWith(Encoding.UTF8.GetPreamble()));
        }

        [Fact]
        public void a_pin_used_by_another_class_fails_naming_both()
        {
            var error = Assert.Throws<BuildException>(
                () => Assign(Obj("Objects.B", "unit", pin: "h0AB"), Obj("Objects.A", "unit", pin: "h0AB")));

            Assert.Equal("Objects.A and Objects.B both have the id h0AB", error.Message);
        }

        [Fact]
        public void a_pin_of_another_classes_kept_id_fails_naming_both()
        {
            Assign(Obj("Objects.A", "unit"));

            var error = Assert.Throws<BuildException>(
                () => Assign(Obj("Objects.A", "unit"), Obj("Objects.B", "unit", pin: "h001")));

            Assert.Equal("Objects.A and Objects.B both have the id h001", error.Message);
        }

        [Fact]
        public void the_same_id_in_two_spaces_is_fine()
        {
            var ids = Assign(Obj("Objects.Burn", "buff", pin: "B0AA"), Obj("Objects.Gate", "destructable", pin: "B0AA"));

            Assert.Equal("B0AA", ids["Objects.Burn"]);
            Assert.Equal("B0AA", ids["Objects.Gate"]);
        }

        [Fact]
        public void a_pin_that_is_not_four_characters_fails()
        {
            var error = Assert.Throws<BuildException>(() => Assign(Obj("Objects.A", "unit", pin: "h01")));

            Assert.Equal("Objects.A pins the id \"h01\", which is not four characters", error.Message);
        }

        [Fact]
        public void a_pin_of_a_standard_id_fails()
        {
            var error = Assert.Throws<BuildException>(
                () => Assign(new HashSet<string> { "hfoo" }, None, None, Obj("Objects.A", "unit", pin: "hfoo")));

            Assert.Equal("Objects.A pins the id hfoo, which a standard object has", error.Message);
        }

        [Fact]
        public void a_malformed_file_fails_naming_it()
        {
            var path = Path.Combine(folder, "ids.json");
            File.WriteAllText(path, "{ \"Objects.A\": ");

            var error = Assert.Throws<BuildException>(() => Assign(Obj("Objects.A", "unit")));

            Assert.StartsWith(path + " is not a JSON object from class name to id: ", error.Message);
        }

        [Fact]
        public void a_file_with_a_value_that_is_not_an_id_fails_naming_it()
        {
            var path = Path.Combine(folder, "ids.json");
            File.WriteAllText(path, "{ \"Objects.A\": 5 }");

            var error = Assert.Throws<BuildException>(() => Assign(Obj("Objects.A", "unit")));

            Assert.StartsWith(path + " is not a JSON object from class name to id: ", error.Message);
        }

        [Fact]
        public void no_objects_and_no_file_writes_no_file()
        {
            Assign();

            Assert.False(File.Exists(Path.Combine(folder, "ids.json")));
        }

        private static DeclaredObject Obj(string name, string kind, string root = "hfoo", bool replaces = false, string? pin = null)
        {
            return new DeclaredObject(
                name,
                kind,
                root,
                replaces,
                pin,
                new Dictionary<string, object?>(),
                Array.Empty<string>());
        }

        private IReadOnlyDictionary<string, string> Assign(params DeclaredObject[] objects)
        {
            return Assign(None, None, None, objects);
        }

        private IReadOnlyDictionary<string, string> Assign(
            IReadOnlySet<string> standardIds,
            IReadOnlySet<string> sourceMapIds,
            IReadOnlySet<string> placedIds,
            params DeclaredObject[] objects)
        {
            log.GetStringBuilder().Clear();
            return ObjectIds.Assign(MapProject.Find(folder), objects, standardIds, sourceMapIds, placedIds, log);
        }

        private Dictionary<string, string> ReadFile()
        {
            var text = File.ReadAllText(Path.Combine(folder, "ids.json"));
            return JsonSerializer.Deserialize<Dictionary<string, string>>(text)!;
        }
    }
}
