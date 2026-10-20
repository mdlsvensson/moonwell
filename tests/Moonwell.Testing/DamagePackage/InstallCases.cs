using Moonwell.Hosting;

namespace Moonwell.Testing
{
    /// <summary>Cases that check when the damage package listens for the game's damage events.</summary>
    public static class InstallCases
    {
        /// <summary>Installed, with no reaction anywhere, the package does not listen.</summary>
        public static void NoTriggerWithoutNeed(FakeWorld world)
        {
            var fake = new FakeDamageWorld();
            DamageSystem.Install(fake);
            ObjectTypes.Unit(ObjectTypes.FourCC("h000"), () => new Unit());
            Expect.True(!fake.Listening, "the package does not listen");
        }

        /// <summary>A unit class that implements IDamageTarget makes the package listen.</summary>
        public static void ListensForAnInterface(FakeWorld world)
        {
            var fake = new FakeDamageWorld();
            DamageSystem.Install(fake);
            ObjectTypes.Unit(ObjectTypes.FourCC("h000"), () => new Taker());
            Expect.True(fake.Listening, "the package listens");
        }

        /// <summary>A class that overrides core's OnDamaged makes the package listen, and core does not listen itself.</summary>
        public static void ListensForOnDamaged(FakeWorld world)
        {
            var fake = new FakeDamageWorld();
            DamageSystem.Install(fake);
            ObjectTypes.Unit(ObjectTypes.FourCC("h000"), () => new Dummy());
            Expect.True(fake.Listening, "the package listens");
            Expect.True(!world.Listening.Contains(GameEvent.Damage), "core does not listen for Damage");
        }
    }

    /// <summary>A unit that logs the hits it takes after armor.</summary>
    public class Taker : Unit, IDamageTarget
    {
        /// <summary>Logs the hit's amount.</summary>
        public void OnTaken(Hit hit)
        {
            TestLog.Lines.Add("taker taken " + hit.Amount);
        }
    }
}
