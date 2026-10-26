using Moonwell;
using Moonwell.Objects;

namespace InputFixture
{
    /// <summary>At the start: the first player's camera moves to World Editor's "Intro" camera, and a hero follows that player's mouse.</summary>
    public sealed class Tour : GameComponent
    {
        public override void OnGameStart()
        {
            var hero = Unit.Create<Human.Units.Paladin>(Player.One, 0f, 0f);
            Player.One.Camera().Apply(CameraSetup.Named("Intro"), 2f);
            Player.One.Add(new Aim(hero));
        }
    }
}
