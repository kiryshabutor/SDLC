// Рисунок А.3 — макет: вход, регистрация, восстановление пароля (REQ-FUNC-001…REQ-FUNC-004)
package UI_01_Auth {
    private import UiKit::*;

    part <auth> 'Экраны гостя' : Row {
        part 'Вход' : Screen {
            part 'Email [ivan@mail.by]' : Input;
            part 'Пароль [••••••••]' : Input;
            part '⚠ Неверный email или пароль' : Label;
            part 'Войти' : Button;
            part 'Регистрация' : Link;
            part 'Забыли пароль?' : Link;
        }
        part 'Регистрация' : Screen {
            part 'Email [ivan@mail.by]' : Input;
            part 'Пароль [••••••••]' : Input;
            part 'Повтор пароля [••••••••]' : Input;
            part 'Не менее 8 символов, буква и цифра' : Label;
            part 'Регион магазина: Беларусь (BY) ▾' : Select;
            part '☐ Согласен на обработку персональных данных' : Checkbox;
            part 'Зарегистрироваться' : Button;
            part 'Письмо для подтверждения действует 24 ч' : Label;
            part 'Уже есть учётная запись? Войти' : Link;
        }
        part 'Восстановление пароля' : Screen {
            part 'Шаг 1. Запрос ссылки' : Form {
                part 'Email [ivan@mail.by]' : Input;
                part 'Отправить ссылку' : Button;
                part 'Если email зарегистрирован, письмо отправлено' : Label;
            }
            part 'Шаг 2. По ссылке из письма (1 ч, однократно)' : Form {
                part 'Новый пароль [••••••••]' : Input;
                part 'Повтор пароля [••••••••]' : Input;
                part 'Сохранить пароль' : Button;
            }
        }
    }
}
