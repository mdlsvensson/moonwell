namespace Moonwell.Build.Tests
{
    public sealed class ObjectIdsTests
    {
        private static readonly IReadOnlySet<string> None = new HashSet<string>();

        [Fact]
        public void ids_come_from_the_attribute()
        {
            var ids = Assign(
                Obj("Objects.Arthas", "hero", pin: "H00A"),
                Obj("Objects.Footman", "unit", pin: "h00F"),
                Obj("Objects.Bolt", "ability", pin: "A00B"));

            Assert.Equal("H00A", ids["Objects.Arthas"]);
            Assert.Equal("h00F", ids["Objects.Footman"]);
            Assert.Equal("A00B", ids["Objects.Bolt"]);
            Assert.Equal(3, ids.Count);
        }

        [Fact]
        public void carrier_comes_from_the_attribute()
        {
            var ids = Assign(Obj("Objects.Burn", "buff", pin: "B001", carrier: "A002"));

            Assert.Equal("B001", ids["Objects.Burn"]);
            Assert.Equal("A002", ids["Objects.Burn#carrier"]);
        }

        [Fact]
        public void missing_id_fails_naming_a_free_one()
        {
            var error = Assert.Throws<BuildException>(() => Assign(
                new HashSet<string> { "H001" },
                new HashSet<string> { "H002" },
                Obj("Objects.Arthas", "hero"),
                Obj("Objects.Other", "hero", pin: "H003")));

            Assert.Equal("Objects.Arthas has no object id; add [ObjectId(\"H004\")] to it", error.Message);
        }

        [Fact]
        public void every_missing_id_is_named_each_with_its_own_free_id()
        {
            var error = Assert.Throws<BuildException>(() => Assign(
                Obj("Objects.Footman", "unit"),
                Obj("Objects.Grunt", "unit"),
                Obj("Objects.Burn", "buff")));

            Assert.Equal(
                "Objects.Burn has no object id; add [ObjectId(\"B001\", Carrier = \"A001\")] to it\n"
                + "Objects.Footman has no object id; add [ObjectId(\"h001\")] to it\n"
                + "Objects.Grunt has no object id; add [ObjectId(\"h002\")] to it",
                error.Message);
        }

        [Fact]
        public void buff_without_carrier_fails_naming_a_free_ability_id()
        {
            var error = Assert.Throws<BuildException>(() => Assign(
                Obj("Objects.Bolt", "ability", pin: "A001"),
                Obj("Objects.Burn", "buff", pin: "B001")));

            Assert.Equal("Objects.Burn has no carrier id; add Carrier = \"A002\" to its [ObjectId]", error.Message);
        }

        [Fact]
        public void carrier_on_a_non_buff_fails()
        {
            var error = Assert.Throws<BuildException>(() => Assign(Obj("Objects.Bolt", "ability", pin: "A001", carrier: "A002")));

            Assert.Equal("Objects.Bolt is not a buff, so its [ObjectId] cannot give a carrier", error.Message);
        }

        [Fact]
        public void hero_id_must_start_uppercase()
        {
            var error = Assert.Throws<BuildException>(() => Assign(Obj("Objects.Arthas", "hero", pin: "h001")));

            Assert.Equal("Objects.Arthas is a hero, so its id h001 must start with an upper-case letter", error.Message);
        }

        [Fact]
        public void unit_id_must_start_lowercase()
        {
            var error = Assert.Throws<BuildException>(() => Assign(Obj("Objects.Footman", "unit", pin: "H001")));

            Assert.Equal("Objects.Footman is a unit, so its id H001 must not start with an upper-case letter: the game makes such a unit a hero", error.Message);
        }

        [Fact]
        public void a_pin_that_is_not_four_characters_fails()
        {
            var error = Assert.Throws<BuildException>(() => Assign(Obj("Objects.Arthas", "hero", pin: "H01")));

            Assert.Equal("Objects.Arthas has the id \"H01\", which is not four characters", error.Message);
        }

        [Fact]
        public void a_pin_of_a_standard_id_fails()
        {
            var error = Assert.Throws<BuildException>(() => Assign(new HashSet<string> { "Hpal" }, None, Obj("Objects.Arthas", "hero", pin: "Hpal")));

            Assert.Equal("Objects.Arthas has the id Hpal, which a standard object has", error.Message);
        }

        [Fact]
        public void two_classes_with_one_id_fail_naming_both()
        {
            var error = Assert.Throws<BuildException>(() => Assign(Obj("Objects.B", "hero", pin: "H001"), Obj("Objects.A", "hero", pin: "H001")));

            Assert.Equal("Objects.A and Objects.B both have the id H001", error.Message);
        }

        [Fact]
        public void a_carrier_with_an_ability_id_fails_naming_both()
        {
            var error = Assert.Throws<BuildException>(() => Assign(
                Obj("Objects.Bolt", "ability", pin: "A001"),
                Obj("Objects.Burn", "buff", pin: "B001", carrier: "A001")));

            Assert.Equal("Objects.Bolt and Objects.Burn#carrier both have the id A001", error.Message);
        }

        [Fact]
        public void the_same_id_in_two_spaces_is_fine()
        {
            var ids = Assign(Obj("Objects.Burn", "buff", pin: "B001", carrier: "A001"), Obj("Objects.Gate", "destructable", pin: "B001"));

            Assert.Equal("B001", ids["Objects.Burn"]);
            Assert.Equal("B001", ids["Objects.Gate"]);
        }

        [Fact]
        public void replaces_takes_standard_id()
        {
            var ids = Assign(Obj("Objects.Footman", "unit", root: "hfoo", replaces: true), Obj("Objects.Other", "unit", pin: "h001"));

            Assert.Equal("hfoo", ids["Objects.Footman"]);
            Assert.Equal("h001", ids["Objects.Other"]);
        }

        [Fact]
        public void replaces_cannot_pin()
        {
            var error = Assert.Throws<BuildException>(
                () => Assign(Obj("Objects.Footman", "unit", root: "hfoo", replaces: true, pin: "h001")));

            Assert.Equal("Objects.Footman replaces hfoo and cannot have an id of its own", error.Message);
        }

        private static DeclaredObject Obj(
            string name,
            string kind,
            string root = "hfoo",
            bool replaces = false,
            string? pin = null,
            string? carrier = null)
        {
            return new DeclaredObject(
                name,
                kind,
                root,
                replaces,
                pin,
                carrier,
                new Dictionary<string, object?>(),
                Array.Empty<string>());
        }

        private static IReadOnlyDictionary<string, string> Assign(params DeclaredObject[] objects)
        {
            return Assign(None, None, objects);
        }

        private static IReadOnlyDictionary<string, string> Assign(
            IReadOnlySet<string> standardIds,
            IReadOnlySet<string> sourceMapIds,
            params DeclaredObject[] objects)
        {
            return ObjectIds.Assign(objects, standardIds, sourceMapIds);
        }
    }
}
