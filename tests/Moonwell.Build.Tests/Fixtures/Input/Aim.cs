using Moonwell;

namespace InputFixture
{
    /// <summary>Turns a hero to the cursor on a right click.</summary>
    public sealed class Aim : PlayerComponent, IMouseWatch
    {
        private readonly Unit hero;

        public Aim(Unit hero)
        {
            this.hero = hero;
        }

        public void OnMousePressed(Mouse mouse, MouseButton button)
        {
            if (button == MouseButton.Right)
            {
                hero.FaceNow(hero.AngleTo(mouse.Position));
            }
        }
    }
}
