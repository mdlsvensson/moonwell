using System.Collections.Generic;

namespace Moonwell.Testing
{
    /// <summary>Cases that check a hero: its experience, level, stats, skill points and revive.</summary>
    public static class HeroCases
    {
        /// <summary>Makes a Paladin the game made as a hero, with no class registered for its type.</summary>
        private static Hero NewHero(FakeWorld world, out unit handle)
        {
            handle = world.NewUnit(ObjectTypes.FourCC("Hpal"), hero: true);
            return (Hero)Unit.Of(handle)!;
        }

        /// <summary>A hero handle without a registered class gives a Hero; a unit that is not a hero does not.</summary>
        public static void UnregisteredIsHero(FakeWorld world)
        {
            var handle = world.NewUnit(ObjectTypes.FourCC("Hpal"), hero: true);
            var found = Unit.Of(handle);
            Expect.True(found is Hero, "the unit of a hero handle is a Hero");
            Expect.Equal("Hero", Names.NameOf(found!), "the class of the hero");
            Expect.True(!(Unit.Of(world.NewUnit(ObjectTypes.FourCC("hfoo"))) is Hero), "a footman is not a Hero");
        }

        /// <summary>Experience reads the game's and sets it with the level-up effect; the game is asked once per set.</summary>
        public static void Experience(FakeWorld world)
        {
            var hero = NewHero(world, out var handle);
            Expect.Equal(0, hero.Experience, "the experience before");

            hero.Experience = 500;

            Expect.Equal(500, hero.Experience, "the experience after");
            Expect.Equal(500, world.ExperienceOf(handle), "the game's experience");
            Expect.Equal(1, world.HeroCalls.Count, "calls that changed the hero");
            Expect.Equal("experience 500 True", world.HeroCalls[0], "the call");
        }

        /// <summary>SetLevel gives the hero the level in the game, with the effect unless it is turned off; Level reads it.</summary>
        public static void Level(FakeWorld world)
        {
            var hero = NewHero(world, out var handle);
            Expect.Equal(1, hero.Level, "the level before");

            hero.SetLevel(5);

            Expect.Equal(5, hero.Level, "the level after");
            hero.SetLevel(8, false);
            Expect.Equal(8, hero.Level, "the level after the second");
            Expect.Equal(2, world.HeroCalls.Count, "calls that changed the hero");
            Expect.Equal("level 5 True", world.HeroCalls[0], "the first call");
            Expect.Equal("level 8 False", world.HeroCalls[1], "the second call");
        }

        /// <summary>The stats are the game's base values, and With Bonus adds what items and auras give; setting changes the base for good.</summary>
        public static void Stats(FakeWorld world)
        {
            var hero = NewHero(world, out var handle);
            world.SetStats(handle, 22, 13, 17);
            world.SetStatBonuses(handle, 4, 5, 6);

            Expect.Equal(22, hero.Strength, "strength");
            Expect.Equal(13, hero.Agility, "agility");
            Expect.Equal(17, hero.Intelligence, "intelligence");
            Expect.Equal(26, hero.StrengthWithBonus, "strength with bonus");
            Expect.Equal(18, hero.AgilityWithBonus, "agility with bonus");
            Expect.Equal(23, hero.IntelligenceWithBonus, "intelligence with bonus");
            Expect.Equal(0, world.HeroCalls.Count, "reading asks nothing to change");

            hero.Strength = 30;
            hero.Agility = 31;
            hero.Intelligence = 32;

            Expect.Equal(30, hero.Strength, "strength after");
            Expect.Equal(31, hero.Agility, "agility after");
            Expect.Equal(32, hero.Intelligence, "intelligence after");
            Expect.Equal(34, hero.StrengthWithBonus, "strength with bonus after");
            Expect.Equal(36, hero.AgilityWithBonus, "agility with bonus after");
            Expect.Equal(38, hero.IntelligenceWithBonus, "intelligence with bonus after");
            Expect.Equal(3, world.HeroCalls.Count, "calls that changed the hero");
            Expect.Equal("strength 30 True", world.HeroCalls[0], "the strength call is permanent");
            Expect.Equal("agility 31 True", world.HeroCalls[1], "the agility call is permanent");
            Expect.Equal("intelligence 32 True", world.HeroCalls[2], "the intelligence call is permanent");
        }

        /// <summary>SkillPoints reads the game's, and setting it adds or takes the difference.</summary>
        public static void SkillPoints(FakeWorld world)
        {
            var hero = NewHero(world, out var handle);
            world.SetSkillPoints(handle, 2);
            Expect.Equal(2, hero.SkillPoints, "the points before");

            hero.SkillPoints = 5;
            Expect.Equal(5, hero.SkillPoints, "the points after raising");
            Expect.Equal("skill-points 3", world.HeroCalls[0], "the difference added");

            hero.SkillPoints = 1;
            Expect.Equal(1, hero.SkillPoints, "the points after lowering");
            Expect.Equal("skill-points -4", world.HeroCalls[1], "the difference taken");

            hero.SkillPoints = 1;
            Expect.Equal(2, world.HeroCalls.Count, "setting the same value asks nothing");
        }

        /// <summary>A dead hero revives at the place, with the effect unless it is turned off, and is alive; a living hero is not revived and the game is not asked.</summary>
        public static void Revive(FakeWorld world)
        {
            var hero = NewHero(world, out var handle);
            Expect.True(!hero.Revive(10.0f, 20.0f), "a living hero is not revived");
            Expect.Equal(0, world.HeroCalls.Count, "the game is not asked about a living hero");

            hero.Kill();
            Expect.True(!hero.IsAlive, "dead after Kill");

            Expect.True(hero.Revive(100.0f, 200.0f), "the dead hero is revived");

            Expect.True(hero.IsAlive, "alive after Revive");
            Expect.Near(100.0f, hero.X, "x after Revive");
            Expect.Near(200.0f, hero.Y, "y after Revive");
            Expect.Equal(1, world.HeroCalls.Count, "calls that changed the hero");
            Expect.Equal("revive 100 200 True", world.HeroCalls[0], "the call");

            hero.Kill();
            Expect.True(hero.Revive(5.0f, 6.0f, false), "revived again");
            Expect.Equal("revive 5 6 False", world.HeroCalls[1], "the call without the effect");
        }

        /// <summary>A hero the game has removed gives the values it last read or set, ignores setters, and Revive says false; the game is not asked.</summary>
        public static void Gone(FakeWorld world)
        {
            var hero = NewHero(world, out var handle);
            world.SetStats(handle, 22, 13, 17);
            world.SetStatBonuses(handle, 4, 5, 6);
            world.SetSkillPoints(handle, 2);
            hero.Experience = 700;
            hero.SetLevel(6);
            Expect.Equal(22, hero.Strength, "strength read");
            Expect.Equal(13, hero.Agility, "agility read");
            Expect.Equal(17, hero.Intelligence, "intelligence read");
            Expect.Equal(26, hero.StrengthWithBonus, "strength with bonus read");
            Expect.Equal(18, hero.AgilityWithBonus, "agility with bonus read");
            Expect.Equal(23, hero.IntelligenceWithBonus, "intelligence with bonus read");
            Expect.Equal(2, hero.SkillPoints, "skill points read");
            Expect.Equal(6, hero.Level, "level read");

            // Set and never read: the setters keep the value.
            var unread = NewHero(world, out var unreadHandle);
            unread.Experience = 300;
            unread.Strength = 40;
            unread.Agility = 41;
            unread.Intelligence = 42;
            unread.SkillPoints = 4;

            world.VanishUnit(handle);
            world.VanishUnit(unreadHandle);
            world.Advance(0.3f);
            Expect.True(hero.IsGone && unread.IsGone, "the sweep found both gone");
            world.GoneCalls = 0;
            var calls = world.HeroCalls.Count;

            hero.Experience = 1;
            hero.SetLevel(9);
            hero.Strength = 1;
            hero.Agility = 1;
            hero.Intelligence = 1;
            hero.SkillPoints = 9;

            Expect.True(!hero.Revive(1.0f, 2.0f), "a gone hero is not revived");
            Expect.Equal(700, hero.Experience, "experience");
            Expect.Equal(6, hero.Level, "level");
            Expect.Equal(22, hero.Strength, "strength");
            Expect.Equal(13, hero.Agility, "agility");
            Expect.Equal(17, hero.Intelligence, "intelligence");
            Expect.Equal(26, hero.StrengthWithBonus, "strength with bonus");
            Expect.Equal(18, hero.AgilityWithBonus, "agility with bonus");
            Expect.Equal(23, hero.IntelligenceWithBonus, "intelligence with bonus");
            Expect.Equal(2, hero.SkillPoints, "skill points");
            Expect.Equal(300, unread.Experience, "unread experience");
            Expect.Equal(40, unread.Strength, "unread strength");
            Expect.Equal(41, unread.Agility, "unread agility");
            Expect.Equal(42, unread.Intelligence, "unread intelligence");
            Expect.Equal(4, unread.SkillPoints, "unread skill points");
            Expect.Equal(0, world.GoneCalls, "no call to the game for the gone hero");
            Expect.Equal(calls, world.HeroCalls.Count, "no call that changes the hero");
        }


        /// <summary>The game removes a hero and every number reads 0 before the sweep finds it (seen 10-21): the stats, experience, level and skill points stay as last read.</summary>
        public static void GoneBeforeSweep(FakeWorld world)
        {
            var hero = NewHero(world, out var handle);
            world.SetStats(handle, 22, 13, 17);
            world.SetStatBonuses(handle, 4, 5, 6);
            world.SetSkillPoints(handle, 2);
            hero.Experience = 700;
            hero.SetLevel(6);
            Expect.Equal(22, hero.Strength, "strength read");
            Expect.Equal(13, hero.Agility, "agility read");
            Expect.Equal(17, hero.Intelligence, "intelligence read");
            Expect.Equal(26, hero.StrengthWithBonus, "strength with bonus read");
            Expect.Equal(18, hero.AgilityWithBonus, "agility with bonus read");
            Expect.Equal(23, hero.IntelligenceWithBonus, "intelligence with bonus read");
            Expect.Equal(2, hero.SkillPoints, "skill points read");
            Expect.Equal(700, hero.Experience, "experience read");
            Expect.Equal(6, hero.Level, "level read");

            world.VanishUnit(handle);

            Expect.True(!hero.IsGone, "the sweep has not run");
            Expect.Equal(22, hero.Strength, "strength");
            Expect.Equal(13, hero.Agility, "agility");
            Expect.Equal(17, hero.Intelligence, "intelligence");
            Expect.Equal(26, hero.StrengthWithBonus, "strength with bonus");
            Expect.Equal(18, hero.AgilityWithBonus, "agility with bonus");
            Expect.Equal(23, hero.IntelligenceWithBonus, "intelligence with bonus");
            Expect.Equal(2, hero.SkillPoints, "skill points");
            Expect.Equal(700, hero.Experience, "experience");
            Expect.Equal(6, hero.Level, "level");
            world.Advance(0.3f);
            Expect.True(hero.IsGone, "the sweep found the hero gone");
            Expect.Equal(22, hero.Strength, "strength after the sweep");
            Expect.Equal(700, hero.Experience, "experience after the sweep");
        }

        /// <summary>A hero's proper name is the game's: read inside Local.For, set for every machine; once the hero is gone, the last name read or set, and a set calls nothing.</summary>
        public static void ProperName(FakeWorld world)
        {
            var hero = NewHero(world, out var handle);
            world.SetProperName(handle, "Uther");
            world.HeroCalls.Clear();
            var seen = "";
            Local.For(Player.Get(0), () => seen = hero.ProperName);
            Expect.Equal("Uther", seen, "the game's proper name");

            hero.ProperName = "Arthas";

            Local.For(Player.Get(0), () => seen = hero.ProperName);
            Expect.Equal("Arthas", seen, "the new proper name");
            Expect.Equal("Arthas", world.ProperNameOf(handle), "the game's proper name after the set");
            Expect.Equal(1, world.HeroCalls.Count, "calls that changed the hero");
            Expect.Equal("proper-name Arthas", world.HeroCalls[0], "the call");

            hero.Remove();
            hero.ProperName = "Later";

            Local.For(Player.Get(0), () => seen = hero.ProperName);
            Expect.Equal("Arthas", seen, "the gone hero's last proper name");
            Expect.Equal(1, world.HeroCalls.Count, "no call for the gone hero");
        }

        /// <summary>A hero that records what its shared members ask of the hooks, and reads each declared value as Unit does.</summary>
        private sealed class RecordingHero : Hero
        {
            /// <summary>The hook calls, in turn, as "read" or "declare", the field id, and the value declared.</summary>
            public List<string> Asked = new List<string>();

            /// <summary>Records the read, and reads the value as Unit does.</summary>
            protected override T DeclaredValue<T>(string field)
            {
                Asked.Add("read " + field);
                return base.DeclaredValue<T>(field);
            }

            /// <summary>Records the declared value.</summary>
            protected override void Declare(string field, object value)
            {
                Asked.Add("declare " + field + " " + value);
            }
        }

        /// <summary>A hero with no handle calls no native for its stats: a set goes to Declare and a read to DeclaredValue.</summary>
        public static void SharedWithoutHandle(FakeWorld world)
        {
            var id = ObjectTypes.FourCC("H0rc");
            ObjectTypes.Unit(id, () => new RecordingHero());
            world.SetTypeValue(id, "ustr", 22);
            var hero = new RecordingHero();
            world.CallsThrow = true;

            hero.Strength = 30;
            hero.Agility = 31;
            hero.Intelligence = 32;
            var strength = hero.Strength;
            var agility = hero.Agility;
            var intelligence = hero.Intelligence;

            world.CallsThrow = false;
            Expect.Equal(22, strength, "the strength its type declares");
            Expect.Equal(0, agility, "an agility its type does not declare");
            Expect.Equal(0, intelligence, "intelligence");
            var expected = new string[] { "declare ustr 30", "declare uagi 31", "declare uint 32", "read ustr", "read uagi", "read uint" };
            Expect.Equal(expected.Length, hero.Asked.Count, "hook calls");
            for (var i = 0; i < expected.Length; i++)
            {
                Expect.Equal(expected[i], hero.Asked[i], "hook call " + i);
            }
        }
    }
}
