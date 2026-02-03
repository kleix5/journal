

import json
from datetime import date
import os


def default_dict():
    with open("names_list.json", 'r', encoding="utf-8") as f:
        return json.load(f)

def presense_check(field: dict):
    '''Отмечает присутствующих по списку.
    ДЕЛАЕМ ОДИН РАЗ'''
    for k, v in field.items():
        user_input = input(f"Отмечаем {k}\n")
        if user_input == '+':
            field[k]["presense"] = True

    # for i in range(len(groupe)):
    #     print(f"{i}, {groupe[i]}")

        
def presense_set(field: dict, name: str):
    '''отмечает опоздавших'''
    field[name]["presense"] = True

def mark_set(field: dict, name: str, mark: int):
    '''Ставит оценки.'''
    field[name]["mark"] = mark

def student_choice(number: int, field: dict) -> str:
    check_list = []
    count = 1
    for k, v in field.items():
        check_list.append((count, k))
        count += 1
    for i in check_list:
        if i[0] == number:
            return i[1] 


def teacher_action(field: dict):
    display_student_list(field)
    inp = int(input("Введите номер:\n"))
    return inp


def display_student_list(field: dict):
    check_list = []
    count = 1
    for k, v in field.items():
        check_list.append((count, k))
        count += 1
    for i in check_list:
        print(i[0], i[1]) 
        
def save_state(field: dict):
    '''создаём файл и записываем туда словарь'''
    flag = 'w'
    if "jstest.json" in os.listdir():
        flag = 'a'
    current_date = date.today()
    # написать проверку наличия файла в папке
    with open("jstest.json", flag) as f:
        date_dict = {str(current_date): field}
        json.dump(date_dict, f)

def read_state(file: str) -> dict:
    """Читает данные пользователя из файла. Если файла нет — создаёт."""
    try:
        with open(file, "r+", encoding="utf-8") as f:
            return json.load(f)
    except FileNotFoundError:
        print("Файл не найден!")



def show(field: dict):
    """Отображает журнал"""
    for k, v in field.items():
        print(k, v)


